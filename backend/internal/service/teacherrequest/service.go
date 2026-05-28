// Package teacherrequest реализует жизненный цикл заявок пользователей
// на получение роли преподавателя:
//   - просмотр pending-заявок деканатом;
//   - одобрение (автоматически назначает роль teacher);
//   - отклонение с причиной.
package teacherrequest

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

var (
	ErrNotFound        = errors.New("teacher_request: заявка не найдена")
	ErrAlreadyReviewed = errors.New("teacher_request: заявка уже рассмотрена")
)

// Service управляет заявками на роль преподавателя.
type Service struct {
	store *repo.Store
}

func New(store *repo.Store) *Service {
	return &Service{store: store}
}

// WithUser объединяет сырую заявку с полями пользователя-заявителя —
// чтобы не делать N+1 запросов в handler.
type WithUser struct {
	queries.TeacherRoleRequest
	Email      string
	FirstName  string
	LastName   string
	MiddleName *string
}

// ListPending возвращает все pending-заявки с данными пользователей.
func (s *Service) ListPending(ctx context.Context, limit, offset int32) ([]WithUser, error) {
	rows, err := s.store.ListPendingTeacherRoleRequests(ctx,
		queries.ListPendingTeacherRoleRequestsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list pending teacher requests: %w", err)
	}

	out := make([]WithUser, 0, len(rows))
	for _, r := range rows {
		user, err := s.store.GetUserByID(ctx, r.RequestedBy)
		if err != nil {
			if repo.IsNotFound(err) {
				out = append(out, WithUser{TeacherRoleRequest: r})
				continue
			}
			return nil, fmt.Errorf("get user %s: %w", pgutil.UUID(r.RequestedBy), err)
		}
		out = append(out, WithUser{
			TeacherRoleRequest: r,
			Email:              user.Email,
			FirstName:          user.FirstName,
			LastName:           user.LastName,
			MiddleName:         user.MiddleName,
		})
	}
	return out, nil
}

// Approve одобряет заявку и в той же транзакции выдаёт пользователю
// роль teacher.
func (s *Service) Approve(ctx context.Context, requestID, actorID uuid.UUID) (queries.TeacherRoleRequest, error) {
	req, err := s.store.GetTeacherRoleRequestByID(ctx, pgutil.PgUUID(requestID))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.TeacherRoleRequest{}, ErrNotFound
		}
		return queries.TeacherRoleRequest{}, fmt.Errorf("get teacher request: %w", err)
	}
	if req.Status != "pending" {
		return queries.TeacherRoleRequest{}, ErrAlreadyReviewed
	}

	var result queries.TeacherRoleRequest
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		updated, err := q.ApproveTeacherRoleRequest(ctx, queries.ApproveTeacherRoleRequestParams{
			ReviewedBy: pgutil.PgUUID(actorID),
			ID:         pgutil.PgUUID(requestID),
		})
		if err != nil {
			return fmt.Errorf("approve: %w", err)
		}
		result = updated

		// Проверяем, нет ли уже этой роли, чтобы не нарушить UNIQUE-индекс.
		already, err := q.HasRole(ctx, queries.HasRoleParams{
			UserID: req.RequestedBy,
			Lower:  "teacher",
		})
		if err != nil {
			return fmt.Errorf("check role: %w", err)
		}
		if !already {
			teacherRole, err := q.GetRoleBySlug(ctx, "teacher")
			if err != nil {
				return fmt.Errorf("get teacher role: %w", err)
			}
			if _, err := q.AttachRoleToUser(ctx, queries.AttachRoleToUserParams{
				UserID:    req.RequestedBy,
				RoleID:    teacherRole.ID,
				CreatedBy: pgutil.PgUUID(actorID),
			}); err != nil {
				return fmt.Errorf("attach teacher role: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return queries.TeacherRoleRequest{}, err
	}
	return result, nil
}

// Reject отклоняет заявку с обязательной причиной.
func (s *Service) Reject(ctx context.Context, requestID, actorID uuid.UUID, reason string) (queries.TeacherRoleRequest, error) {
	req, err := s.store.GetTeacherRoleRequestByID(ctx, pgutil.PgUUID(requestID))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.TeacherRoleRequest{}, ErrNotFound
		}
		return queries.TeacherRoleRequest{}, fmt.Errorf("get teacher request: %w", err)
	}
	if req.Status != "pending" {
		return queries.TeacherRoleRequest{}, ErrAlreadyReviewed
	}

	updated, err := s.store.RejectTeacherRoleRequest(ctx, queries.RejectTeacherRoleRequestParams{
		ReviewedBy:     pgutil.PgUUID(actorID),
		DecisionReason: reason,
		ID:             pgutil.PgUUID(requestID),
	})
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.TeacherRoleRequest{}, ErrNotFound
		}
		return queries.TeacherRoleRequest{}, fmt.Errorf("reject: %w", err)
	}
	return updated, nil
}
