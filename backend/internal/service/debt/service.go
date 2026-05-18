// Package debt реализует управление академическими долгами:
// постановка долга преподавателем, выставление оценки (закрытие),
// отмена деканатом, выборки по ролям (свои / по дисциплинам / все).
//
// Валидационные инварианты (по ТЗ):
//   - долг ставит только преподаватель, ведущий эту дисциплину;
//   - долг ставится только студенту, который учится на дисциплине;
//   - один открытый долг на (student, discipline) — UNIQUE-индекс
//     в БД (idx_debts_unique_open), сервис ловит pgconn.UniqueViolation
//     и возвращает ErrDuplicateOpenDebt;
//   - переход open → graded требует final_grade ∈ {2,3,4,5} и
//     фиксирует graded_at/graded_by (БД-CHECK debts_grade_consistency);
//   - переход open → cancelled (без оценки) — для деканата.
//
// Каждая мутация идёт через Store.RunInTx с записью audit_log +
// change_logs в той же транзакции.
package debt

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
)

const (
	entityType = "debt"

	statusOpen      = "open"
	statusGraded    = "graded"
	statusCancelled = "cancelled"

	sourceManual = "manual"

	actionCreated   = "debt.created"
	actionGraded    = "debt.graded"
	actionCancelled = "debt.cancelled"
)

// Sentinel-ошибки.
var (
	ErrNotFound           = errors.New("debt: не найден")
	ErrInvalidInput       = errors.New("debt: некорректные параметры")
	ErrTeacherNotAssigned = errors.New("debt: преподаватель не ведёт эту дисциплину")
	ErrStudentNotEnrolled = errors.New("debt: студент не учится на этой дисциплине")
	ErrDuplicateOpenDebt  = errors.New("debt: открытый долг по этой дисциплине уже существует")
	ErrNotOpen            = errors.New("debt: долг не в статусе open")
	ErrInvalidGrade       = errors.New("debt: оценка должна быть в диапазоне 2..5")
)

// Service инкапсулирует операции с долгами.
type Service struct {
	store       *repo.Store
	audit       *audit.Service
	changelog   *changelog.Service
	disciplines *discipline.Service
}

// New собирает Service. discipline нужен для валидации
// "преподаватель ведёт?" / "студент учится?" — это бизнес-инварианты
// уровня создания долга.
func New(store *repo.Store, auditSvc *audit.Service, changelogSvc *changelog.Service, disciplinesSvc *discipline.Service) *Service {
	return &Service{
		store:       store,
		audit:       auditSvc,
		changelog:   changelogSvc,
		disciplines: disciplinesSvc,
	}
}

// CreateInput — параметры постановки долга. issued_by приходит из
// контекста запроса (текущий пользователь-преподаватель).
type CreateInput struct {
	StudentID    uuid.UUID
	DisciplineID uuid.UUID
	ExternalID   *string
	Source       string // 'manual' (default) или 'sync'
	Notes        *string
}

// Create ставит долг. Делает три проверки до RunInTx, потому что
// они дешевле, чем разбирать pgconn-ошибку UNIQUE/FK после INSERT.
func (s *Service) Create(ctx context.Context, in CreateInput, issuedBy uuid.UUID) (queries.Debt, error) {
	if in.StudentID == uuid.Nil || in.DisciplineID == uuid.Nil {
		return queries.Debt{}, fmt.Errorf("%w: student_id и discipline_id обязательны", ErrInvalidInput)
	}
	source := in.Source
	if source == "" {
		source = sourceManual
	}
	if source != "manual" && source != "sync" {
		return queries.Debt{}, fmt.Errorf("%w: source должен быть manual или sync", ErrInvalidInput)
	}

	// Преподаватель ведёт дисциплину?
	isTeacher, err := s.disciplines.IsTeacherOf(ctx, issuedBy, in.DisciplineID)
	if err != nil {
		return queries.Debt{}, fmt.Errorf("check teacher: %w", err)
	}
	if !isTeacher {
		return queries.Debt{}, ErrTeacherNotAssigned
	}

	// Студент учится на дисциплине?
	enrolled, err := s.disciplines.IsStudentEnrolled(ctx, in.StudentID, in.DisciplineID)
	if err != nil {
		return queries.Debt{}, fmt.Errorf("check enrollment: %w", err)
	}
	if !enrolled {
		return queries.Debt{}, ErrStudentNotEnrolled
	}

	var created queries.Debt
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.CreateDebt(ctx, queries.CreateDebtParams{
			StudentID:    pgutil.PgUUID(in.StudentID),
			DisciplineID: pgutil.PgUUID(in.DisciplineID),
			IssuedBy:     pgutil.PgUUID(issuedBy),
			ExternalID:   in.ExternalID,
			Source:       source,
			Notes:        in.Notes,
		})
		if err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicateOpenDebt
			}
			return fmt.Errorf("insert debt: %w", err)
		}
		created = row

		if err := s.changelog.LogCreatedTx(ctx, q, entityType, pgutil.UUID(row.ID).String(),
			debtFields(row), issuedBy); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    issuedBy,
			Action:     actionCreated,
			TargetType: entityType,
			TargetID:   pgutil.UUID(row.ID).String(),
			Details: map[string]any{
				"student_id":    in.StudentID.String(),
				"discipline_id": in.DisciplineID.String(),
			},
		})
	})
	if err != nil {
		return queries.Debt{}, err
	}
	return created, nil
}

// Get возвращает долг по id. soft-deleted → ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (queries.Debt, error) {
	row, err := s.store.GetDebtByID(ctx, pgutil.PgUUID(id))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.Debt{}, ErrNotFound
		}
		return queries.Debt{}, fmt.Errorf("get debt: %w", err)
	}
	return row, nil
}

// Grade переводит open → graded. Преподаватель, который ведёт
// дисциплину долга, имеет право поставить оценку.
//
// Если переход не сработал (status уже не open или запись удалена) —
// возвращаем ErrNotOpen, чтобы фронт показал понятное сообщение.
func (s *Service) Grade(ctx context.Context, id uuid.UUID, grade int32, gradedBy uuid.UUID) (queries.Debt, error) {
	if grade < 2 || grade > 5 {
		return queries.Debt{}, ErrInvalidGrade
	}

	current, err := s.Get(ctx, id)
	if err != nil {
		return queries.Debt{}, err
	}
	if current.Status != statusOpen {
		return queries.Debt{}, ErrNotOpen
	}

	// Преподаватель должен вести эту дисциплину (а не быть случайным
	// преподом из системы).
	isTeacher, err := s.disciplines.IsTeacherOf(ctx, gradedBy, pgutil.UUID(current.DisciplineID))
	if err != nil {
		return queries.Debt{}, fmt.Errorf("check teacher: %w", err)
	}
	if !isTeacher {
		return queries.Debt{}, ErrTeacherNotAssigned
	}

	var updated queries.Debt
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.GradeDebt(ctx, queries.GradeDebtParams{
			ID:         current.ID,
			FinalGrade: &grade,
			GradedBy:   pgutil.PgUUID(gradedBy),
		})
		if err != nil {
			if repo.IsNotFound(err) {
				// WHERE-условие в UPDATE не нашло строку — статус уже изменился.
				return ErrNotOpen
			}
			return fmt.Errorf("grade debt: %w", err)
		}
		updated = row

		if err := s.changelog.LogUpdatedTx(ctx, q, entityType, pgutil.UUID(row.ID).String(),
			debtFields(current), debtFields(row), gradedBy); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    gradedBy,
			Action:     actionGraded,
			TargetType: entityType,
			TargetID:   pgutil.UUID(row.ID).String(),
			Details:    map[string]any{"final_grade": grade},
		})
	})
	if err != nil {
		return queries.Debt{}, err
	}
	return updated, nil
}

// Cancel переводит open → cancelled. По ТЗ это право деканата
// (debts.delete), проверка permission — на handler-слое.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, cancelledBy uuid.UUID) error {
	current, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != statusOpen {
		return ErrNotOpen
	}

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.CancelDebt(ctx, current.ID); err != nil {
			return fmt.Errorf("cancel debt: %w", err)
		}
		// Перечитываем для фиксации before/after в changelog.
		after, err := q.GetDebtByID(ctx, current.ID)
		if err != nil {
			return fmt.Errorf("reload debt: %w", err)
		}
		if err := s.changelog.LogUpdatedTx(ctx, q, entityType, pgutil.UUID(after.ID).String(),
			debtFields(current), debtFields(after), cancelledBy); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    cancelledBy,
			Action:     actionCancelled,
			TargetType: entityType,
			TargetID:   pgutil.UUID(after.ID).String(),
		})
	})
}

// debtFields формирует JSON-friendly срез для change_logs. Точно
// перечисляем, какие поля документируем — чтобы случайно не
// засунуть в лог что-то лишнее.
func debtFields(d queries.Debt) changelog.Fields {
	f := changelog.Fields{
		"id":            pgutil.UUID(d.ID).String(),
		"student_id":    pgutil.UUID(d.StudentID).String(),
		"discipline_id": pgutil.UUID(d.DisciplineID).String(),
		"issued_by":     pgutil.UUID(d.IssuedBy).String(),
		"status":        d.Status,
		"source":        d.Source,
	}
	if d.FinalGrade != nil {
		f["final_grade"] = *d.FinalGrade
	}
	if d.GradedAt.Valid {
		f["graded_at"] = d.GradedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if d.GradedBy.Valid {
		f["graded_by"] = pgutil.UUID(d.GradedBy).String()
	}
	if d.ExternalID != nil {
		f["external_id"] = *d.ExternalID
	}
	if d.Notes != nil {
		f["notes"] = *d.Notes
	}
	return f
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
