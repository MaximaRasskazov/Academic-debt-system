// Package retake реализует управление пересдачами: создание, обновление
// расписания, отмену, ручные переходы статусов и работу с участниками.
//
// Бизнес-инварианты (по ТЗ):
//   - kind='commission' требует минимум 3 преподавателей в составе —
//     проверяется при попытке перевести пересдачу в in_progress;
//   - переходы статусов: scheduled → in_progress → completed (вручную
//     в этом сервисе, авто-переходы по времени делает шедулер в BACK-07);
//   - cancelled — только из scheduled или in_progress;
//   - выставление оценки на пересдаче атомарно закрывает связанный долг
//     (debts.status → 'graded') в одной транзакции.
//
// Все мутации идут через Store.RunInTx с записью audit_log и change_logs.
package retake

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
)

// Константы статусов и видов пересдач (соответствуют CHECK-constraint
// в миграции 00015_create_retakes.sql).
const (
	entityType = "retake"

	KindRegular    = "regular"
	KindCommission = "commission"

	StatusScheduled  = "scheduled"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusCancelled  = "cancelled"

	// MinCommissionTeachers — требование ТЗ "комиссионная пересдача
	// требует минимум 3 преподавателей". При создании пересдачи с
	// kind=commission поле min_teachers выставляется в это значение.
	MinCommissionTeachers = 3
	MinRegularTeachers    = 1

	actionCreated        = "retake.created"
	actionScheduleUpdate = "retake.schedule_updated"
	actionStarted        = "retake.started"
	actionCompleted      = "retake.completed"
	actionCancelled      = "retake.cancelled"
)

// Sentinel-ошибки. HTTP-слой маппит их в 400/404/409/422.
var (
	ErrNotFound          = errors.New("retake: не найдена")
	ErrInvalidInput      = errors.New("retake: некорректные параметры")
	ErrInvalidKind       = errors.New("retake: kind должен быть regular или commission")
	ErrInvalidStatus     = errors.New("retake: переход статуса невозможен")
	ErrNotEnoughTeachers = errors.New("retake: для комиссии нужно минимум 3 преподавателя")
	ErrAlreadyHasGrade   = errors.New("retake: оценка уже выставлена")
)

// Service инкапсулирует операции с пересдачами и их участниками.
//
// При выставлении оценки на пересдаче связанный долг закрывается в
// той же транзакции — для этого retake-сервис дёргает q.GradeDebt
// напрямую в RunInTx-callback, не через debt-сервис. Это сознательное
// нарушение strict layering ради атомарности (нельзя смешать tx двух
// сервисов без сложной transactional-functor машинерии).
type Service struct {
	store     *repo.Store
	audit     *audit.Service
	changelog *changelog.Service
}

func New(store *repo.Store, auditSvc *audit.Service, changelogSvc *changelog.Service) *Service {
	return &Service{store: store, audit: auditSvc, changelog: changelogSvc}
}

// CreateInput — параметры создания пересдачи.
//
// При kind=commission MinTeachers игнорируется и принудительно
// выставляется 3 — БД-CHECK retakes_commission_min_teachers иначе
// отвергнет INSERT.
type CreateInput struct {
	DisciplineID    uuid.UUID
	Kind            string // 'regular' или 'commission'
	Building        string
	Room            string
	ScheduledAt     pgutilTime // см. ниже helper
	DurationMinutes int32
	Notes           *string
}

// Create создаёт пересдачу со статусом scheduled. Участников нужно
// добавлять отдельно через AddStudent / AddTeacher.
func (s *Service) Create(ctx context.Context, in CreateInput, actorID uuid.UUID) (queries.Retake, error) {
	if in.DisciplineID == uuid.Nil {
		return queries.Retake{}, fmt.Errorf("%w: discipline_id обязателен", ErrInvalidInput)
	}
	building := strings.TrimSpace(in.Building)
	room := strings.TrimSpace(in.Room)
	if building == "" || room == "" {
		return queries.Retake{}, fmt.Errorf("%w: building и room обязательны", ErrInvalidInput)
	}
	if in.DurationMinutes <= 0 {
		return queries.Retake{}, fmt.Errorf("%w: duration_minutes должна быть положительной", ErrInvalidInput)
	}
	if !in.ScheduledAt.Valid {
		return queries.Retake{}, fmt.Errorf("%w: scheduled_at обязателен", ErrInvalidInput)
	}

	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = KindRegular
	}
	var minTeachers int32
	switch kind {
	case KindRegular:
		minTeachers = MinRegularTeachers
	case KindCommission:
		minTeachers = MinCommissionTeachers
	default:
		return queries.Retake{}, ErrInvalidKind
	}

	var created queries.Retake
	err := s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.CreateRetake(ctx, queries.CreateRetakeParams{
			DisciplineID:    pgutil.PgUUID(in.DisciplineID),
			Kind:            kind,
			MinTeachers:     minTeachers,
			Building:        building,
			Room:            room,
			ScheduledAt:     in.ScheduledAt.toTimestamptz(),
			DurationMinutes: in.DurationMinutes,
			CreatedBy:       pgutil.PgUUID(actorID),
			Notes:           in.Notes,
		})
		if err != nil {
			return fmt.Errorf("create retake: %w", err)
		}
		created = row

		if err := s.changelog.LogCreatedTx(ctx, q, entityType, pgutil.UUID(row.ID).String(),
			retakeFields(row), actorID); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionCreated,
			TargetType: entityType,
			TargetID:   pgutil.UUID(row.ID).String(),
			Details:    map[string]any{"kind": kind, "discipline_id": in.DisciplineID.String()},
		})
	})
	if err != nil {
		return queries.Retake{}, err
	}
	return created, nil
}

// Get возвращает пересдачу. ErrNotFound — на soft-deleted тоже.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (queries.Retake, error) {
	row, err := s.store.GetRetakeByID(ctx, pgutil.PgUUID(id))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.Retake{}, ErrNotFound
		}
		return queries.Retake{}, fmt.Errorf("get retake: %w", err)
	}
	return row, nil
}

// UpdateScheduleInput — PATCH-семантика, поля nil не меняются.
type UpdateScheduleInput struct {
	Building        *string
	Room            *string
	ScheduledAt     *pgutilTime
	DurationMinutes *int32
	Notes           *string
}

// UpdateSchedule меняет место/время/продолжительность пересдачи.
// Используется при ручной правке деканатом или при одобрении
// retake_change_request (см. BACK-04).
func (s *Service) UpdateSchedule(ctx context.Context, id uuid.UUID, in UpdateScheduleInput, actorID uuid.UUID) (queries.Retake, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return queries.Retake{}, err
	}
	if current.Status == StatusCompleted || current.Status == StatusCancelled {
		return queries.Retake{}, fmt.Errorf("%w: пересдача завершена или отменена", ErrInvalidStatus)
	}
	if in.DurationMinutes != nil && *in.DurationMinutes <= 0 {
		return queries.Retake{}, fmt.Errorf("%w: duration_minutes должна быть положительной", ErrInvalidInput)
	}

	var updated queries.Retake
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		params := queries.UpdateRetakeScheduleParams{ID: current.ID, Notes: in.Notes}
		if in.Building != nil {
			b := strings.TrimSpace(*in.Building)
			params.Building = &b
		}
		if in.Room != nil {
			rm := strings.TrimSpace(*in.Room)
			params.Room = &rm
		}
		if in.ScheduledAt != nil {
			params.ScheduledAt = in.ScheduledAt.toTimestamptz()
		}
		if in.DurationMinutes != nil {
			params.DurationMinutes = in.DurationMinutes
		}

		row, err := q.UpdateRetakeSchedule(ctx, params)
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrNotFound
			}
			return fmt.Errorf("update schedule: %w", err)
		}
		updated = row

		if err := s.changelog.LogUpdatedTx(ctx, q, entityType, pgutil.UUID(row.ID).String(),
			retakeFields(current), retakeFields(row), actorID); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionScheduleUpdate,
			TargetType: entityType,
			TargetID:   pgutil.UUID(row.ID).String(),
		})
	})
	if err != nil {
		return queries.Retake{}, err
	}
	return updated, nil
}

// Start переводит пересдачу из scheduled в in_progress. Проверяет,
// что у пересдачи с kind=commission достаточно преподавателей (≥3).
// Обычно вызывается шедулером (BACK-07), но доступен и вручную.
func (s *Service) Start(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	current, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != StatusScheduled {
		return fmt.Errorf("%w: можно стартовать только из scheduled", ErrInvalidStatus)
	}

	count, err := s.store.CountTeachersInRetake(ctx, current.ID)
	if err != nil {
		return fmt.Errorf("count teachers: %w", err)
	}
	if count < int64(current.MinTeachers) {
		return ErrNotEnoughTeachers
	}

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.MarkRetakeInProgress(ctx, current.ID); err != nil {
			return fmt.Errorf("mark in_progress: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionStarted,
			TargetType: entityType,
			TargetID:   pgutil.UUID(current.ID).String(),
		})
	})
}

// Complete переводит в completed. Совместимо как с scheduled (если
// время уже прошло, но никто не Start'нул), так и с in_progress.
func (s *Service) Complete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	current, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != StatusScheduled && current.Status != StatusInProgress {
		return fmt.Errorf("%w: завершить можно только активную пересдачу", ErrInvalidStatus)
	}

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.MarkRetakeCompleted(ctx, current.ID); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionCompleted,
			TargetType: entityType,
			TargetID:   pgutil.UUID(current.ID).String(),
		})
	})
}

// Cancel отменяет пересдачу. Только из scheduled / in_progress.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	current, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != StatusScheduled && current.Status != StatusInProgress {
		return fmt.Errorf("%w: отменить можно только активную пересдачу", ErrInvalidStatus)
	}

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.CancelRetake(ctx, current.ID); err != nil {
			return fmt.Errorf("cancel: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionCancelled,
			TargetType: entityType,
			TargetID:   pgutil.UUID(current.ID).String(),
		})
	})
}

// retakeFields формирует JSON-friendly срез полей для change_logs.
func retakeFields(r queries.Retake) changelog.Fields {
	f := changelog.Fields{
		"id":               pgutil.UUID(r.ID).String(),
		"discipline_id":    pgutil.UUID(r.DisciplineID).String(),
		"kind":             r.Kind,
		"min_teachers":     r.MinTeachers,
		"building":         r.Building,
		"room":             r.Room,
		"scheduled_at":     r.ScheduledAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		"duration_minutes": r.DurationMinutes,
		"status":           r.Status,
	}
	if r.CompletedAt.Valid {
		f["completed_at"] = r.CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if r.Notes != nil {
		f["notes"] = *r.Notes
	}
	return f
}
