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
// дисциплине вместе с именами. Используется на главном дашборде
// деканата как агрегат, без пагинации (дисциплин в учебном году
// обычно десятки).
func (s *Service) DebtsSummary(ctx context.Context) ([]DisciplineSummary, error) {
	rows, err := s.store.SummaryDebtsByDiscipline(ctx)
	if err != nil {
		return nil, fmt.Errorf("summary by discipline: %w", err)
	}

	out := make([]DisciplineSummary, 0, len(rows))
	for _, r := range rows {
		summary := DisciplineSummary{
			DisciplineID: pgutil.UUID(r.DisciplineID),
			OpenCount:    r.OpenCount,
			GradedCount:  r.GradedCount,
		}
		// Подтягиваем имя/код одиночными запросами. N+1, но дисциплин
		// мало (десятки) и кешировать пока избыточно.
		disc, err := s.store.GetDisciplineByID(ctx, r.DisciplineID)
		if err == nil {
			summary.DisciplineName = disc.Name
			summary.DisciplineCode = disc.Code
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
// архива защит/отчётности.
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

		// Дисциплина.
		disc, err := s.store.GetDisciplineByID(ctx, r.DisciplineID)
		if err == nil {
			row.DisciplineName = disc.Name
			row.DisciplineCode = disc.Code
		}

		// Студенты с расширенной инфой через специальный JOIN-запрос.
		students, err := s.store.ListStudentParticipantsForRetake(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("list students for retake %s: %w", pgutil.UUID(r.ID), err)
		}
		for _, p := range students {
			row.Students = append(row.Students, ParticipantInfo{
				UserID:    pgutil.UUID(p.UserID),
				FullName:  fullName(p.LastName, p.FirstName, p.MiddleName),
				GroupName: p.GroupName,
				Email:     p.Email,
				Grade:     p.Grade,
			})
		}

		// Преподаватели + комиссия — без email/group, только имя.
		teachers, err := s.store.ListTeacherParticipantsForRetake(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("list teachers for retake %s: %w", pgutil.UUID(r.ID), err)
		}
		for _, p := range teachers {
			user, err := s.store.GetUserByID(ctx, p.UserID)
			if err != nil {
				// Пропускаем участника без user — это data integrity
				// проблема, но не повод ломать весь отчёт.
				continue
			}
			row.Teachers = append(row.Teachers, ParticipantInfo{
				UserID:   pgutil.UUID(p.UserID),
				FullName: fullName(user.LastName, user.FirstName, user.MiddleName),
				Email:    user.Email,
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
