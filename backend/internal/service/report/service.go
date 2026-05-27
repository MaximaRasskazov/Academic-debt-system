// Package report формирует сводные отчёты для деканата:
//   - DebtsSummary: количество открытых/закрытых долгов по каждой
//     дисциплине (для главного дашборда деканата);
//   - RetakesForPeriod: подробный список проведённых пересдач за
//     указанный промежуток с участниками и оценками (входит в выгрузку
//     XLSX/CSV для архива).
//
// По требованию permission reports.export — на handler-слое.
package report

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

var (
	// ErrInvalidPeriod возвращается, если from >= to. Защита от
	// "пустых" и "перевёрнутых" диапазонов.
	ErrInvalidPeriod = errors.New("report: from должен быть раньше to")
)

// Service инкапсулирует чтение агрегатов и сборку детальных отчётов.
// stateless, зависит только от *repo.Store.
type Service struct {
	store *repo.Store
}

func New(store *repo.Store) *Service {
	return &Service{store: store}
}

// DisciplineSummary — строка сводки долгов по одной дисциплине.
// Поле DisciplineName может быть пустой строкой, если дисциплина была
// soft-deleted после постановки долга — это нормально, в выгрузке
// показываем placeholder "(дисциплина удалена)".
type DisciplineSummary struct {
	DisciplineID   uuid.UUID
	DisciplineName string
	DisciplineCode string
	OpenCount      int64
	GradedCount    int64
}

// DebtsSummary возвращает сводку открытых/закрытых долгов по каждой
// дисциплине вместе с именами. Использует батч-запрос вместо N+1.
func (s *Service) DebtsSummary(ctx context.Context) ([]DisciplineSummary, error) {
	rows, err := s.store.SummaryDebtsByDiscipline(ctx)
	if err != nil {
		return nil, fmt.Errorf("summary by discipline: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	// Собираем все ID дисциплин одним срезом для батч-запроса.
	ids := make([]pgtype.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.DisciplineID
	}

	discs, err := s.store.ListDisciplinesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list disciplines by ids: %w", err)
	}
	discMap := make(map[pgtype.UUID]queries.Discipline, len(discs))
	for _, d := range discs {
		discMap[d.ID] = d
	}

	out := make([]DisciplineSummary, 0, len(rows))
	for _, r := range rows {
		summary := DisciplineSummary{
			DisciplineID: pgutil.UUID(r.DisciplineID),
			OpenCount:    r.OpenCount,
			GradedCount:  r.GradedCount,
		}
		if d, ok := discMap[r.DisciplineID]; ok {
			summary.DisciplineName = d.Name
			summary.DisciplineCode = d.Code
		}
		// Если дисциплина soft-deleted — оставляем пустые поля, чтобы
		// в отчёте отразился factual count "долг был по неактуальной
		// дисциплине". Это лучше чем скрывать данные.
		out = append(out, summary)
	}
	return out, nil
}

// RetakeReportRow — одна строка детального отчёта по пересдаче за
// период. Собирается в Go-сервисе из retake + participants + discipline.
type RetakeReportRow struct {
	RetakeID        uuid.UUID
	DisciplineName  string
	DisciplineCode  string
	Kind            string // regular / commission
	Building        string
	Room            string
	ScheduledAt     time.Time
	CompletedAt     time.Time
	DurationMinutes int32
	Students        []ParticipantInfo // те, кому ставилась оценка
	Teachers        []ParticipantInfo // teacher + commission_member
}

// ParticipantInfo — компактная инфа об участнике для отчёта.
// Если Grade nil — оценки нет (студент не явился, например).
type ParticipantInfo struct {
	UserID    uuid.UUID
	FullName  string  // last + first + (middle)
	GroupName *string // только у студентов
	Email     string
	Grade     *int32 // только у студентов после проставления
}

// RetakesForPeriod возвращает детальный список проведённых пересдач
// за период [from, to). Включает только status='completed' — для
// архива защит/отчётности. Использует батч-запросы вместо N+1.
func (s *Service) RetakesForPeriod(ctx context.Context, from, to time.Time) ([]RetakeReportRow, error) {
	if !from.Before(to) {
		return nil, ErrInvalidPeriod
	}

	retakes, err := s.store.ListRetakesInPeriod(ctx, queries.ListRetakesInPeriodParams{
		From: pgtype.Timestamptz{Time: from, Valid: true},
		To:   pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list retakes in period: %w", err)
	}
	if len(retakes) == 0 {
		return nil, nil
	}

	// --- батч дисциплин ---
	discIDs := make([]pgtype.UUID, len(retakes))
	for i, r := range retakes {
		discIDs[i] = r.DisciplineID
	}
	discs, err := s.store.ListDisciplinesByIDs(ctx, discIDs)
	if err != nil {
		return nil, fmt.Errorf("list disciplines for retakes: %w", err)
	}
	discMap := make(map[pgtype.UUID]queries.Discipline, len(discs))
	for _, d := range discs {
		discMap[d.ID] = d
	}

	// --- участники и батч пользователей ---
	// Сначала собираем всех участников по всем пересдачам,
	// потом одним запросом тянем нужных преподавателей.
	type retakeParticipants struct {
		students []queries.ListStudentParticipantsForRetakeRow
		teachers []queries.RetakeParticipant
	}
	participantsMap := make(map[pgtype.UUID]retakeParticipants, len(retakes))

	teacherUserIDSet := make(map[pgtype.UUID]struct{})
	for _, r := range retakes {
		students, err := s.store.ListStudentParticipantsForRetake(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("list students for retake %s: %w", pgutil.UUID(r.ID), err)
		}
		teachers, err := s.store.ListTeacherParticipantsForRetake(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("list teachers for retake %s: %w", pgutil.UUID(r.ID), err)
		}
		participantsMap[r.ID] = retakeParticipants{students: students, teachers: teachers}
		for _, t := range teachers {
			teacherUserIDSet[t.UserID] = struct{}{}
		}
	}

	// Батч-выборка всех преподавателей одним запросом.
	teacherIDs := make([]pgtype.UUID, 0, len(teacherUserIDSet))
	for id := range teacherUserIDSet {
		teacherIDs = append(teacherIDs, id)
	}
	userMap := make(map[pgtype.UUID]queries.User)
	if len(teacherIDs) > 0 {
		teacherUsers, err := s.store.ListUsersByIDs(ctx, teacherIDs)
		if err != nil {
			return nil, fmt.Errorf("list teacher users: %w", err)
		}
		for _, u := range teacherUsers {
			userMap[u.ID] = u
		}
	}

	// --- сборка результата ---
	out := make([]RetakeReportRow, 0, len(retakes))
	for _, r := range retakes {
		row := RetakeReportRow{
			RetakeID:        pgutil.UUID(r.ID),
			Kind:            r.Kind,
			Building:        r.Building,
			Room:            r.Room,
			ScheduledAt:     r.ScheduledAt.Time,
			CompletedAt:     r.CompletedAt.Time,
			DurationMinutes: r.DurationMinutes,
		}
		if d, ok := discMap[r.DisciplineID]; ok {
			row.DisciplineName = d.Name
			row.DisciplineCode = d.Code
		}

		p := participantsMap[r.ID]
		for _, st := range p.students {
			row.Students = append(row.Students, ParticipantInfo{
				UserID:    pgutil.UUID(st.UserID),
				FullName:  fullName(st.LastName, st.FirstName, st.MiddleName),
				GroupName: st.GroupName,
				Email:     st.Email,
				Grade:     st.Grade,
			})
		}
		for _, t := range p.teachers {
			u, ok := userMap[t.UserID]
			if !ok {
				// Пропускаем участника без user — data integrity проблема,
				// но не повод ломать весь отчёт.
				continue
			}
			row.Teachers = append(row.Teachers, ParticipantInfo{
				UserID:   pgutil.UUID(t.UserID),
				FullName: fullName(u.LastName, u.FirstName, u.MiddleName),
				Email:    u.Email,
			})
		}
		out = append(out, row)
	}
	return out, nil
}

// fullName собирает ФИО в формате "Фамилия Имя Отчество".
// Отчество опционально (у иностранных студентов может не быть).
func fullName(last, first string, middle *string) string {
	if middle != nil && *middle != "" {
		return last + " " + first + " " + *middle
	}
	return last + " " + first
}
