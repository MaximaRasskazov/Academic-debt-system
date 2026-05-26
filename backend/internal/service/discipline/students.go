package discipline

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
)

// AttachStudentInput — параметры привязки студента к дисциплине.
// academic_year и semester опциональны, но обычно заполняются (для
// фильтров отчётов "за 2025-2026, 2-й семестр").
type AttachStudentInput struct {
	AcademicYear *string
	Semester     *int32 // 1 или 2 (CHECK в БД)
	Source       string // 'manual' (по умолчанию) или 'sync'
}

// AttachStudent создаёт связку студент↔дисциплина.
func (s *Service) AttachStudent(ctx context.Context, disciplineID, studentID, actorID uuid.UUID, in AttachStudentInput) error {
	if _, err := s.Get(ctx, disciplineID); err != nil {
		return err
	}
	source := in.Source
	if source == "" {
		source = sourceManual
	}
	if source != "manual" && source != "sync" {
		return fmt.Errorf("%w: source должен быть manual или sync", ErrInvalidInput)
	}
	if in.Semester != nil && *in.Semester != 1 && *in.Semester != 2 {
		return fmt.Errorf("%w: semester должен быть 1 или 2", ErrInvalidInput)
	}

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		_, err := q.AttachStudentToDiscipline(ctx, queries.AttachStudentToDisciplineParams{
			StudentID:    pgutil.PgUUID(studentID),
			DisciplineID: pgutil.PgUUID(disciplineID),
			AcademicYear: in.AcademicYear,
			Semester:     in.Semester,
			Source:       source,
		})
		if err != nil {
			if isUniqueViolation(err) {
				return ErrAlreadyExists
			}
			return fmt.Errorf("attach student: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionStudentAttached,
			TargetType: entityType,
			TargetID:   disciplineID.String(),
			Details: map[string]any{
				"student_id":    studentID.String(),
				"academic_year": in.AcademicYear,
				"semester":      in.Semester,
				"source":        source,
			},
		})
	})
}

// DetachStudent снимает студента с дисциплины (soft-delete связки).
func (s *Service) DetachStudent(ctx context.Context, disciplineID, studentID, actorID uuid.UUID) error {
	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.DetachStudentFromDiscipline(ctx, queries.DetachStudentFromDisciplineParams{
			StudentID:    pgutil.PgUUID(studentID),
			DisciplineID: pgutil.PgUUID(disciplineID),
		}); err != nil {
			return fmt.Errorf("detach student: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionStudentDetached,
			TargetType: entityType,
			TargetID:   disciplineID.String(),
			Details:    map[string]any{"student_id": studentID.String()},
		})
	})
}

// ListStudents — учащиеся на дисциплине.
// ErrNotFound если дисциплина не существует.
func (s *Service) ListStudents(ctx context.Context, disciplineID uuid.UUID) ([]queries.User, error) {
	if _, err := s.Get(ctx, disciplineID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListStudentsInDiscipline(ctx, pgutil.PgUUID(disciplineID))
	if err != nil {
		return nil, fmt.Errorf("list students: %w", err)
	}
	return rows, nil
}

// ListDisciplinesForStudent — дисциплины, на которых учится студент.
func (s *Service) ListDisciplinesForStudent(ctx context.Context, studentID uuid.UUID) ([]queries.Discipline, error) {
	rows, err := s.store.ListDisciplinesForStudent(ctx, pgutil.PgUUID(studentID))
	if err != nil {
		return nil, fmt.Errorf("list disciplines for student: %w", err)
	}
	return rows, nil
}

// IsStudentEnrolled — быстрый predicate для сервиса долгов.
func (s *Service) IsStudentEnrolled(ctx context.Context, studentID, disciplineID uuid.UUID) (bool, error) {
	ok, err := s.store.IsStudentEnrolled(ctx, queries.IsStudentEnrolledParams{
		StudentID:    pgutil.PgUUID(studentID),
		DisciplineID: pgutil.PgUUID(disciplineID),
	})
	if err != nil {
		return false, fmt.Errorf("is enrolled: %w", err)
	}
	return ok, nil
}
