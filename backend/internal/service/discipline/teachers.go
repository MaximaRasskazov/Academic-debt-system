package discipline

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
)

// AttachTeacher привязывает преподавателя к дисциплине. Если привязка
// уже есть (UNIQUE-индекс) — возвращается ErrAlreadyExists для
// читаемой ошибки в API.
func (s *Service) AttachTeacher(ctx context.Context, disciplineID, teacherID, actorID uuid.UUID) error {
	if _, err := s.Get(ctx, disciplineID); err != nil {
		return err
	}
	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		_, err := q.AttachTeacherToDiscipline(ctx, queries.AttachTeacherToDisciplineParams{
			TeacherID:    pgutil.PgUUID(teacherID),
			DisciplineID: pgutil.PgUUID(disciplineID),
			CreatedBy:    pgutil.PgUUID(actorID),
		})
		if err != nil {
			if isUniqueViolation(err) {
				return ErrAlreadyExists
			}
			if repo.IsForeignKeyViolation(err) {
				return fmt.Errorf("%w: пользователь не найден", ErrInvalidInput)
			}
			return fmt.Errorf("attach teacher: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionTeacherAttached,
			TargetType: entityType,
			TargetID:   disciplineID.String(),
			Details:    map[string]any{"teacher_id": teacherID.String()},
		})
	})
}

// DetachTeacher снимает преподавателя с дисциплины (soft-delete связки).
// Если привязки нет — возвращается ErrNotFound, чтобы клиент знал,
// что DELETE был no-op.
func (s *Service) DetachTeacher(ctx context.Context, disciplineID, teacherID, actorID uuid.UUID) error {
	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.DetachTeacherFromDiscipline(ctx, queries.DetachTeacherFromDisciplineParams{
			TeacherID:    pgutil.PgUUID(teacherID),
			DisciplineID: pgutil.PgUUID(disciplineID),
			DeletedBy:    pgutil.PgUUID(actorID),
		}); err != nil {
			return fmt.Errorf("detach teacher: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionTeacherDetached,
			TargetType: entityType,
			TargetID:   disciplineID.String(),
			Details:    map[string]any{"teacher_id": teacherID.String()},
		})
	})
}

// ListTeachers возвращает преподавателей, ведущих дисциплину.
// ErrNotFound если дисциплина не существует.
func (s *Service) ListTeachers(ctx context.Context, disciplineID uuid.UUID) ([]queries.User, error) {
	if _, err := s.Get(ctx, disciplineID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListTeachersForDiscipline(ctx, pgutil.PgUUID(disciplineID))
	if err != nil {
		return nil, fmt.Errorf("list teachers: %w", err)
	}
	return rows, nil
}

// ListDisciplinesForTeacher — обратная выборка: дисциплины конкретного
// преподавателя. Используется в его кабинете и в фильтрах debts.
func (s *Service) ListDisciplinesForTeacher(ctx context.Context, teacherID uuid.UUID) ([]queries.Discipline, error) {
	rows, err := s.store.ListDisciplinesForTeacher(ctx, pgutil.PgUUID(teacherID))
	if err != nil {
		return nil, fmt.Errorf("list disciplines for teacher: %w", err)
	}
	return rows, nil
}

// IsTeacherOf — быстрый predicate для RBAC-фильтров (нужен сервису долгов).
func (s *Service) IsTeacherOf(ctx context.Context, teacherID, disciplineID uuid.UUID) (bool, error) {
	ok, err := s.store.IsTeacherOfDiscipline(ctx, queries.IsTeacherOfDisciplineParams{
		TeacherID:    pgutil.PgUUID(teacherID),
		DisciplineID: pgutil.PgUUID(disciplineID),
	})
	if err != nil {
		return false, fmt.Errorf("is teacher: %w", err)
	}
	return ok, nil
}

// isUniqueViolation проверяет, что ошибка от pgx — это нарушение
// уникального индекса (SQLSTATE 23505). Используется, чтобы переводить
// её в читаемую sentinel-ошибку.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
