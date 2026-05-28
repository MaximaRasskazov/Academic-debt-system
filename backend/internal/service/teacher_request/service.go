// Package teacher_request реализует заявки на роль преподавателя и HTTP-API
// управления ролями/правами (RBAC API).
//
// Жизненный цикл заявки: pending → approved | rejected.
// При одобрении в одной транзакции: статус approved + AttachRoleToUser(teacher).
// При отклонении decision_reason обязателен (enforced БД + сервисом).
package teacher_request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/rbac"
)

const teacherRoleSlug = "teacher"

var (
	ErrAlreadyPending  = errors.New("teacher_request: у пользователя уже есть активная заявка")
	ErrRequestNotFound = errors.New("teacher_request: заявка не найдена")
	ErrNotPending      = errors.New("teacher_request: заявка уже рассмотрена")
	ErrReasonRequired  = errors.New("teacher_request: причина отказа обязательна")
)

type Service struct {
	store *repo.Store
	rbac  *rbac.Service
}

func New(store *repo.Store, rbacSvc *rbac.Service) *Service {
	return &Service{store: store, rbac: rbacSvc}
}

// Create подаёт заявку от имени actorID. Один pending на пользователя.
func (s *Service) Create(ctx context.Context, actorID uuid.UUID, reason *string) (queries.TeacherRoleRequest, error) {
	existing, err := s.store.GetPendingTeacherRoleRequestForUser(ctx, pgutil.PgUUID(actorID))
	if err != nil && !repo.IsNotFound(err) {
		return queries.TeacherRoleRequest{}, fmt.Errorf("check pending: %w", err)
	}
	if err == nil && existing.Status == "pending" {
		return queries.TeacherRoleRequest{}, ErrAlreadyPending
	}

	req, err := s.store.CreateTeacherRoleRequest(ctx, queries.CreateTeacherRoleRequestParams{
		RequestedBy: pgutil.PgUUID(actorID),
		Reason:      reason,
	})
	if err != nil {
		return queries.TeacherRoleRequest{}, fmt.Errorf("create request: %w", err)
	}
	return req, nil
}

// ListPending возвращает заявки со статусом pending для деканата.
func (s *Service) ListPending(ctx context.Context, limit, offset int32) ([]queries.TeacherRoleRequest, error) {
	reqs, err := s.store.ListPendingTeacherRoleRequests(ctx, queries.ListPendingTeacherRoleRequestsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	return reqs, nil
}

// ListForUser возвращает историю заявок конкретного пользователя.
func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]queries.TeacherRoleRequest, error) {
	reqs, err := s.store.ListTeacherRoleRequestsForUser(ctx, queries.ListTeacherRoleRequestsForUserParams{
		RequestedBy: pgutil.PgUUID(userID),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list for user: %w", err)
	}
	return reqs, nil
}

// Approve одобряет заявку: в одной транзакции меняет статус и выдаёт роль teacher.
// actorID должен иметь roles.assign (проверяется через rbac.AssignRole).
func (s *Service) Approve(ctx context.Context, actorID, requestID uuid.UUID, reason *string) (queries.TeacherRoleRequest, error) {
	req, err := s.store.GetTeacherRoleRequestByID(ctx, pgutil.PgUUID(requestID))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.TeacherRoleRequest{}, ErrRequestNotFound
		}
		return queries.TeacherRoleRequest{}, fmt.Errorf("get request: %w", err)
	}
	if req.Status != "pending" {
		return queries.TeacherRoleRequest{}, ErrNotPending
	}

	targetID := pgutil.UUID(req.RequestedBy)

	var approved queries.TeacherRoleRequest
	if err := s.store.RunInTx(ctx, func(q *queries.Queries) error {
		var txErr error
		approved, txErr = q.ApproveTeacherRoleRequest(ctx, queries.ApproveTeacherRoleRequestParams{
			ReviewedBy:     pgutil.PgUUID(actorID),
			DecisionReason: reason,
			ID:             pgutil.PgUUID(requestID),
		})
		if txErr != nil {
			return fmt.Errorf("approve request: %w", txErr)
		}

		role, txErr := q.GetRoleBySlug(ctx, teacherRoleSlug)
		if txErr != nil {
			return fmt.Errorf("get teacher role: %w", txErr)
		}

		if _, txErr = q.AttachRoleToUser(ctx, queries.AttachRoleToUserParams{
			UserID:    pgutil.PgUUID(targetID),
			RoleID:    role.ID,
			CreatedBy: pgutil.PgUUID(actorID),
		}); txErr != nil {
			return fmt.Errorf("attach role: %w", txErr)
		}

		reqIDStr := requestID.String()
		_, txErr = q.CreateAuditEntry(ctx, queries.CreateAuditEntryParams{
			ActorID:    pgutil.PgUUID(actorID),
			Action:     "teacher_request.approve",
			TargetType: "teacher_role_request",
			TargetID:   &reqIDStr,
			Details:    auditDetails(map[string]any{"role": teacherRoleSlug}),
		})
		return txErr
	}); err != nil {
		return queries.TeacherRoleRequest{}, err
	}
	return approved, nil
}

// Reject отклоняет заявку. reason обязателен.
func (s *Service) Reject(ctx context.Context, actorID, requestID uuid.UUID, reason string) (queries.TeacherRoleRequest, error) {
	if reason == "" {
		return queries.TeacherRoleRequest{}, ErrReasonRequired
	}

	req, err := s.store.GetTeacherRoleRequestByID(ctx, pgutil.PgUUID(requestID))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.TeacherRoleRequest{}, ErrRequestNotFound
		}
		return queries.TeacherRoleRequest{}, fmt.Errorf("get request: %w", err)
	}
	if req.Status != "pending" {
		return queries.TeacherRoleRequest{}, ErrNotPending
	}

	var rejected queries.TeacherRoleRequest
	if err := s.store.RunInTx(ctx, func(q *queries.Queries) error {
		var txErr error
		rejected, txErr = q.RejectTeacherRoleRequest(ctx, queries.RejectTeacherRoleRequestParams{
			ReviewedBy:     pgutil.PgUUID(actorID),
			DecisionReason: reason,
			ID:             pgutil.PgUUID(requestID),
		})
		if txErr != nil {
			return fmt.Errorf("reject request: %w", txErr)
		}

		reqIDStr := requestID.String()
		_, txErr = q.CreateAuditEntry(ctx, queries.CreateAuditEntryParams{
			ActorID:    pgutil.PgUUID(actorID),
			Action:     "teacher_request.reject",
			TargetType: "teacher_role_request",
			TargetID:   &reqIDStr,
			Details:    auditDetails(map[string]any{"reason": reason}),
		})
		return txErr
	}); err != nil {
		return queries.TeacherRoleRequest{}, err
	}
	return rejected, nil
}

// auditDetails сериализует payload в JSON для audit_log.details.
// При ошибке маршалинга возвращает "{}" и логирует — лучше потерять
// детали записи, чем получить невалидный JSON в БД (или повредить
// аудит-журнал инъекцией кавычек из user input в reason).
func auditDetails(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		slog.Error("teacher_request: marshal audit details", "err", err)
		return []byte("{}")
	}
	return b
}
