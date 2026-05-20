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
)

// ListForUser — все (не удалённые) пересдачи, в которых пользователь
// участвует. Permission retakes.view.own. Подходит и для студента, и
// для преподавателя — JOIN по retake_participants не различает kind.
func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]queries.Retake, error) {
	rows, err := s.store.ListRetakesForUser(ctx, pgutil.PgUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("list for user: %w", err)
	}
	return rows, nil
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
