package retake

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

const (
	maxLimit     = 200
	defaultLimit = 50

	// teacherRoleSlug — slug роли преподавателя в таблице roles.
	// Совпадает по значению с ParticipantTeacher, но это разные понятия
	// (роль пользователя vs kind участника), поэтому держим отдельно.
	teacherRoleSlug = "teacher"
)

// ListForUser — пересдачи, в которых пользователь участвует (permission
// retakes.view.own). Для преподавателя (роль teacher) показываем только
// пересдачи, где он участвует как teacher/commission_member: иначе у
// бывшего студента, повышенного до преподавателя, в кабинете «протекали»
// бы старые пересдачи, где он был студентом. Для остальных (студент) —
// все его пересдачи.
func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]queries.Retake, error) {
	isTeacher, err := s.store.HasRole(ctx, queries.HasRoleParams{
		UserID: pgutil.PgUUID(userID),
		Lower:  teacherRoleSlug,
	})
	if err != nil {
		return nil, fmt.Errorf("check teacher role: %w", err)
	}

	if isTeacher {
		rows, err := s.store.ListRetakesForUserAsTeacher(ctx, pgutil.PgUUID(userID))
		if err != nil {
			return nil, fmt.Errorf("list for teacher: %w", err)
		}
		return rows, nil
	}

	rows, err := s.store.ListRetakesForUser(ctx, pgutil.PgUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("list for user: %w", err)
	}
	return rows, nil
}

// CanUserAccessRetake сообщает, вправе ли пользователь (без права
// retakes.view.all) видеть состав/ведомость пересдачи. Преподаватель
// получает доступ только к пересдачам, где он участвует как
// teacher/commission_member; студент — где он student. Это закрывает
// «протекание» прошлых студенческих пересдач в кабинет повышенного
// преподавателя.
func (s *Service) CanUserAccessRetake(ctx context.Context, retakeID, userID uuid.UUID) (bool, error) {
	isTeacher, err := s.store.HasRole(ctx, queries.HasRoleParams{
		UserID: pgutil.PgUUID(userID),
		Lower:  teacherRoleSlug,
	})
	if err != nil {
		return false, fmt.Errorf("check teacher role: %w", err)
	}

	kinds := []string{ParticipantStudent}
	if isTeacher {
		kinds = []string{ParticipantTeacher, ParticipantCommissionMember}
	}

	ok, err := s.store.IsRetakeParticipantWithKind(ctx, queries.IsRetakeParticipantWithKindParams{
		RetakeID: pgutil.PgUUID(retakeID),
		UserID:   pgutil.PgUUID(userID),
		Kinds:    kinds,
	})
	if err != nil {
		return false, fmt.Errorf("check participant kind: %w", err)
	}
	return ok, nil
}

// ListAll — общий список для деканата с опциональным фильтром статуса.
// status="" означает "все статусы".
func (s *Service) ListAll(ctx context.Context, status string, limit, offset int32) ([]queries.Retake, error) {
	var statusFilter *string
	if status != "" {
		statusFilter = &status
	}
	rows, err := s.store.ListRetakes(ctx, queries.ListRetakesParams{
		Status: statusFilter,
		Lim:    normalizeLimit(limit),
		Off:    normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list retakes: %w", err)
	}
	return rows, nil
}

// ListInPeriod — для сводных отчётов: только completed-пересдачи в
// указанном промежутке времени (по completed_at).
func (s *Service) ListInPeriod(ctx context.Context, from, to time.Time) ([]queries.Retake, error) {
	rows, err := s.store.ListRetakesInPeriod(ctx, queries.ListRetakesInPeriodParams{
		From: pgtype.Timestamptz{Time: from, Valid: true},
		To:   pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list retakes in period: %w", err)
	}
	return rows, nil
}

func normalizeLimit(v int32) int32 {
	if v <= 0 || v > maxLimit {
		return defaultLimit
	}
	return v
}

func normalizeOffset(v int32) int32 {
	if v < 0 {
		return 0
	}
	return v
}
