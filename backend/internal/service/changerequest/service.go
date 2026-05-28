// Package changerequest реализует жизненный цикл заявок преподавателей
// на изменение расписания пересдачи:
//   - просмотр pending-заявок деканатом;
//   - одобрение (применяет запрошенные изменения к пересдаче);
//   - отклонение с причиной.
package changerequest

import (
	"context"
	"encoding/json"
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
	ErrNotFound        = errors.New("change_request: заявка не найдена")
	ErrAlreadyReviewed = errors.New("change_request: заявка уже рассмотрена")
)

// Service управляет заявками на изменение пересдачи.
type Service struct {
	store *repo.Store
}

func New(store *repo.Store) *Service {
	return &Service{store: store}
}

// requestedChanges — JSON-структура внутри retake_change_requests.requested_changes.
type requestedChanges struct {
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`
	Building        *string    `json:"building,omitempty"`
	Room            *string    `json:"room,omitempty"`
	DurationMinutes *int32     `json:"duration_minutes,omitempty"`
}

// WithUser объединяет сырую заявку с полями пользователя-заявителя.
type WithUser struct {
	queries.RetakeChangeRequest
	RequesterEmail      string
	RequesterFirstName  string
	RequesterLastName   string
	RequesterMiddleName *string
}

// ListPending возвращает все pending-заявки с данными пользователей.
func (s *Service) ListPending(ctx context.Context, limit, offset int32) ([]WithUser, error) {
	rows, err := s.store.ListPendingRetakeChangeRequests(ctx,
		queries.ListPendingRetakeChangeRequestsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list pending change requests: %w", err)
	}

	out := make([]WithUser, 0, len(rows))
	for _, r := range rows {
		user, err := s.store.GetUserByID(ctx, r.RequestedBy)
		if err != nil {
			if repo.IsNotFound(err) {
				out = append(out, WithUser{RetakeChangeRequest: r})
				continue
			}
			return nil, fmt.Errorf("get user %s: %w", pgutil.UUID(r.RequestedBy), err)
		}
		out = append(out, WithUser{
			RetakeChangeRequest:  r,
			RequesterEmail:       user.Email,
			RequesterFirstName:   user.FirstName,
			RequesterLastName:    user.LastName,
			RequesterMiddleName:  user.MiddleName,
		})
	}
	return out, nil
}

// Approve одобряет заявку и в той же транзакции применяет requested_changes
// к соответствующей пересдаче.
func (s *Service) Approve(ctx context.Context, requestID, actorID uuid.UUID) (queries.RetakeChangeRequest, error) {
	req, err := s.store.GetRetakeChangeRequestByID(ctx, pgutil.PgUUID(requestID))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.RetakeChangeRequest{}, ErrNotFound
		}
		return queries.RetakeChangeRequest{}, fmt.Errorf("get change request: %w", err)
	}
	if req.Status != "pending" {
		return queries.RetakeChangeRequest{}, ErrAlreadyReviewed
	}

	// Парсим requested_changes — если JSON кривой, одобряем без применения.
	var changes requestedChanges
	_ = json.Unmarshal(req.RequestedChanges, &changes)

	var result queries.RetakeChangeRequest
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		updated, err := q.ApproveRetakeChangeRequest(ctx, queries.ApproveRetakeChangeRequestParams{
			ReviewedBy: pgutil.PgUUID(actorID),
			ID:         pgutil.PgUUID(requestID),
		})
		if err != nil {
			return fmt.Errorf("approve: %w", err)
		}
		result = updated

		// Применяем изменения только если есть что менять.
		if changes.Building != nil || changes.Room != nil ||
			changes.ScheduledAt != nil || changes.DurationMinutes != nil {
			params := queries.UpdateRetakeScheduleParams{
				ID:              req.RetakeID,
				Building:        changes.Building,
				Room:            changes.Room,
				DurationMinutes: changes.DurationMinutes,
			}
			if changes.ScheduledAt != nil {
				params.ScheduledAt = pgtype.Timestamptz{Time: *changes.ScheduledAt, Valid: true}
			}
			if _, err := q.UpdateRetakeSchedule(ctx, params); err != nil {
				return fmt.Errorf("apply retake changes: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return queries.RetakeChangeRequest{}, err
	}
	return result, nil
}

// Reject отклоняет заявку с обязательной причиной.
func (s *Service) Reject(ctx context.Context, requestID, actorID uuid.UUID, reason string) (queries.RetakeChangeRequest, error) {
	req, err := s.store.GetRetakeChangeRequestByID(ctx, pgutil.PgUUID(requestID))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.RetakeChangeRequest{}, ErrNotFound
		}
		return queries.RetakeChangeRequest{}, fmt.Errorf("get change request: %w", err)
	}
	if req.Status != "pending" {
		return queries.RetakeChangeRequest{}, ErrAlreadyReviewed
	}

	updated, err := s.store.RejectRetakeChangeRequest(ctx, queries.RejectRetakeChangeRequestParams{
		ReviewedBy:     pgutil.PgUUID(actorID),
		DecisionReason: reason,
		ID:             pgutil.PgUUID(requestID),
	})
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.RetakeChangeRequest{}, ErrNotFound
		}
		return queries.RetakeChangeRequest{}, fmt.Errorf("reject: %w", err)
	}
	return updated, nil
}
