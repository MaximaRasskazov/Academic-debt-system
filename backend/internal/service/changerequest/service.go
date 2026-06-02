// Package changerequest реализует заявки преподавателей на изменение
// расписания пересдачи (времени, места, продолжительности).
//
// Жизненный цикл: pending → approved | rejected.
//
// Ключевой инвариант: при одобрении заявки изменения применяются к
// пересдаче в той же транзакции — AtomicApprove вызывает
// q.UpdateRetakeSchedule прямо внутри RunInTx. Это сознательное
// нарушение strict layering ради атомарности: вызов retake.Service
// открыл бы собственную транзакцию, что сломало бы гарантию.
// Тот же паттерн используется в retake.Service.GradeStudent
// (там q.GradeDebt вызывается напрямую).
//
// Все мутации пишут в audit_log. Approve дополнительно пишет в
// change_logs для пересдачи (before/after расписания).
package changerequest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
)

const (
	entityType = "retake_change_request"

	actionSubmitted = "retake_change_request.submitted"
	actionApproved  = "retake_change_request.approved"
	actionRejected  = "retake_change_request.rejected"
)

// Sentinel-ошибки. HTTP-слой маппит их в 400/403/404/409/422.
var (
	ErrNotFound        = errors.New("changerequest: заявка не найдена")
	ErrNotPending      = errors.New("changerequest: заявка уже обработана")
	ErrForbidden       = errors.New("changerequest: преподаватель не является участником пересдачи")
	ErrInvalidInput    = errors.New("changerequest: некорректные параметры")
	ErrRetakeNotActive = errors.New("changerequest: пересдача завершена или отменена")
	// ErrScheduledInPast — новое время пересдачи уже прошло к моменту
	// одобрения: применять нельзя, иначе шедулер сразу переведёт в
	// in_progress («Идёт») вместо ожидаемого «Назначена».
	ErrScheduledInPast = errors.New("changerequest: новое время пересдачи уже прошло")
)

// Changes — поля, которые преподаватель предлагает изменить.
// Хотя бы одно поле должно быть задано (проверяет сервис через hasAny).
// Маршалится в JSONB при сохранении, размаршалится при чтении.
type Changes struct {
	Building        *string    `json:"building,omitempty"`
	Room            *string    `json:"room,omitempty"`
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`
	DurationMinutes *int32     `json:"duration_minutes,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
}

func (c Changes) hasAny() bool {
	return c.Building != nil || c.Room != nil || c.ScheduledAt != nil ||
		c.DurationMinutes != nil || c.Notes != nil
}

// Request — доменная модель заявки. Наружу sqlc-структуры не текут.
type Request struct {
	ID             uuid.UUID
	RetakeID       uuid.UUID
	RequestedBy    uuid.UUID
	Changes        Changes
	Reason         *string
	Status         string
	ReviewedBy     *uuid.UUID
	ReviewedAt     *time.Time
	DecisionReason *string
	CreatedAt      time.Time
	UpdatedAt      *time.Time
}

// Service инкапсулирует операции с заявками на изменение пересдачи.
type Service struct {
	store     *repo.Store
	audit     *audit.Service
	changelog *changelog.Service
	notify    *notify.Service
}

func New(store *repo.Store, a *audit.Service, cl *changelog.Service, n *notify.Service) *Service {
	return &Service{store: store, audit: a, changelog: cl, notify: n}
}

// SubmitInput — параметры подачи заявки.
type SubmitInput struct {
	Changes Changes
	Reason  *string
}

// Submit подаёт заявку от имени actorID. Разрешено только
// преподавателям-участникам пересдачи (kind=teacher или commission_member).
func (s *Service) Submit(ctx context.Context, retakeID, actorID uuid.UUID, in SubmitInput) (Request, error) {
	if !in.Changes.hasAny() {
		return Request{}, fmt.Errorf("%w: необходимо указать хотя бы одно изменение", ErrInvalidInput)
	}

	retake, err := s.store.GetRetakeByID(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		if repo.IsNotFound(err) {
			return Request{}, fmt.Errorf("%w: пересдача не найдена", ErrInvalidInput)
		}
		return Request{}, fmt.Errorf("get retake: %w", err)
	}
	if retake.Status == "completed" || retake.Status == "cancelled" {
		return Request{}, ErrRetakeNotActive
	}

	participant, err := s.store.GetParticipantByRetakeAndUser(ctx, queries.GetParticipantByRetakeAndUserParams{
		RetakeID: pgutil.PgUUID(retakeID),
		UserID:   pgutil.PgUUID(actorID),
	})
	if err != nil {
		if repo.IsNotFound(err) {
			return Request{}, ErrForbidden
		}
		return Request{}, fmt.Errorf("get participant: %w", err)
	}
	if participant.Kind != "teacher" && participant.Kind != "commission_member" {
		return Request{}, ErrForbidden
	}

	changesJSON, err := json.Marshal(in.Changes)
	if err != nil {
		return Request{}, fmt.Errorf("marshal changes: %w", err)
	}

	var created queries.RetakeChangeRequest
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.CreateRetakeChangeRequest(ctx, queries.CreateRetakeChangeRequestParams{
			RetakeID:         pgutil.PgUUID(retakeID),
			RequestedBy:      pgutil.PgUUID(actorID),
			RequestedChanges: changesJSON,
			Reason:           in.Reason,
		})
		if err != nil {
			return fmt.Errorf("create change request: %w", err)
		}
		created = row
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionSubmitted,
			TargetType: entityType,
			TargetID:   pgutil.UUID(row.ID).String(),
			Details:    map[string]any{"retake_id": retakeID.String()},
		})
	})
	if err != nil {
		return Request{}, err
	}
	return fromRow(created)
}

// ListPending возвращает заявки со статусом pending (для деканата).
func (s *Service) ListPending(ctx context.Context, limit, offset int32) ([]Request, error) {
	rows, err := s.store.ListPendingRetakeChangeRequests(ctx, queries.ListPendingRetakeChangeRequestsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	return fromRows(rows)
}

// Get возвращает заявку по id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Request, error) {
	row, err := s.store.GetRetakeChangeRequestByID(ctx, pgutil.PgUUID(id))
	if err != nil {
		if repo.IsNotFound(err) {
			return Request{}, ErrNotFound
		}
		return Request{}, fmt.Errorf("get change request: %w", err)
	}
	return fromRow(row)
}

// Approve одобряет заявку и применяет изменения к пересдаче атомарно.
// В одной транзакции: пометить approved + обновить расписание пересдачи.
// После коммита — best-effort уведомление всех участников.
func (s *Service) Approve(ctx context.Context, id, actorID uuid.UUID, decisionReason *string) (Request, error) {
	req, err := s.Get(ctx, id)
	if err != nil {
		return Request{}, err
	}
	if req.Status != "pending" {
		return Request{}, ErrNotPending
	}

	// Если заявка меняет время — новое время должно быть в будущем,
	// иначе пересдача мгновенно уедет в in_progress по шедулеру.
	if req.Changes.ScheduledAt != nil && !req.Changes.ScheduledAt.After(time.Now()) {
		return Request{}, ErrScheduledInPast
	}

	// Snapshot пересдачи до изменений — для changelog before-state.
	currentRetake, err := s.store.GetRetakeByID(ctx, pgutil.PgUUID(req.RetakeID))
	if err != nil {
		return Request{}, fmt.Errorf("get retake for snapshot: %w", err)
	}

	updateParams := buildUpdateParams(currentRetake.ID, req.Changes)

	var approved queries.RetakeChangeRequest
	var updatedRetake queries.Retake
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.ApproveRetakeChangeRequest(ctx, queries.ApproveRetakeChangeRequestParams{
			ID:             pgutil.PgUUID(id),
			ReviewedBy:     pgutil.PgUUID(actorID),
			DecisionReason: decisionReason,
		})
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrNotPending
			}
			return fmt.Errorf("approve: %w", err)
		}
		approved = row

		updated, err := q.UpdateRetakeSchedule(ctx, updateParams)
		if err != nil {
			return fmt.Errorf("apply schedule changes: %w", err)
		}
		updatedRetake = updated

		if err := s.changelog.LogUpdatedTx(ctx, q, "retake", pgutil.UUID(currentRetake.ID).String(),
			retakeScheduleFields(currentRetake), retakeScheduleFields(updatedRetake), actorID); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionApproved,
			TargetType: entityType,
			TargetID:   pgutil.UUID(approved.ID).String(),
			Details:    map[string]any{"retake_id": req.RetakeID.String()},
		})
	})
	if err != nil {
		return Request{}, err
	}

	s.notifyParticipants(ctx, req.RetakeID, notify.KindRetakeChangeApproved, map[string]any{
		"retake_id":         req.RetakeID.String(),
		"change_request_id": pgutil.UUID(approved.ID).String(),
	})

	return fromRow(approved)
}

// Reject отклоняет заявку. decisionReason обязателен (требование ТЗ).
func (s *Service) Reject(ctx context.Context, id, actorID uuid.UUID, decisionReason string) (Request, error) {
	if strings.TrimSpace(decisionReason) == "" {
		return Request{}, fmt.Errorf("%w: причина отклонения обязательна", ErrInvalidInput)
	}

	req, err := s.Get(ctx, id)
	if err != nil {
		return Request{}, err
	}
	if req.Status != "pending" {
		return Request{}, ErrNotPending
	}

	var rejected queries.RetakeChangeRequest
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.RejectRetakeChangeRequest(ctx, queries.RejectRetakeChangeRequestParams{
			ID:             pgutil.PgUUID(id),
			ReviewedBy:     pgutil.PgUUID(actorID),
			DecisionReason: decisionReason,
		})
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrNotPending
			}
			return fmt.Errorf("reject: %w", err)
		}
		rejected = row
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionRejected,
			TargetType: entityType,
			TargetID:   pgutil.UUID(rejected.ID).String(),
			Details: map[string]any{
				"retake_id":       req.RetakeID.String(),
				"decision_reason": decisionReason,
			},
		})
	})
	if err != nil {
		return Request{}, err
	}

	// Уведомить подавшего заявку об отклонении.
	_ = s.notify.Notify(ctx, notify.Event{
		UserID: req.RequestedBy,
		Kind:   notify.KindRetakeChangeRejected,
		Payload: map[string]any{
			"retake_id":         req.RetakeID.String(),
			"change_request_id": pgutil.UUID(rejected.ID).String(),
			"decision_reason":   decisionReason,
		},
	})

	return fromRow(rejected)
}

// notifyParticipants рассылает уведомление всем участникам пересдачи.
// Best-effort: ошибки логируются, но не возвращаются — не должны
// откатывать уже совершённую транзакцию.
func (s *Service) notifyParticipants(ctx context.Context, retakeID uuid.UUID, kind string, payload map[string]any) {
	parts, err := s.store.ListParticipantsForRetake(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		slog.Warn("changerequest: не удалось получить участников для уведомления",
			"retake_id", retakeID, "err", err)
		return
	}
	for _, p := range parts {
		if err := s.notify.Notify(ctx, notify.Event{
			UserID:  pgutil.UUID(p.UserID),
			Kind:    kind,
			Payload: payload,
		}); err != nil {
			slog.Warn("changerequest: не удалось отправить уведомление",
				"user_id", pgutil.UUID(p.UserID), "err", err)
		}
	}
}

// buildUpdateParams формирует params для q.UpdateRetakeSchedule из Changes.
// Поля nil передаются как NULL — COALESCE в SQL оставит текущее значение.
// ScheduledAt не pointer в params (pgtype.Timestamptz), нулевое значение
// (Valid=false) интерпретируется как NULL → COALESCE оставит старое.
func buildUpdateParams(retakeID pgtype.UUID, c Changes) queries.UpdateRetakeScheduleParams {
	params := queries.UpdateRetakeScheduleParams{ID: retakeID}
	if c.Building != nil {
		b := strings.TrimSpace(*c.Building)
		params.Building = &b
	}
	if c.Room != nil {
		rm := strings.TrimSpace(*c.Room)
		params.Room = &rm
	}
	if c.ScheduledAt != nil {
		params.ScheduledAt = pgtype.Timestamptz{Time: *c.ScheduledAt, Valid: true}
	}
	if c.DurationMinutes != nil {
		params.DurationMinutes = c.DurationMinutes
	}
	params.Notes = c.Notes
	return params
}

// retakeScheduleFields формирует срез расписания для change_logs.
// Включает только поля, которые change request может изменить.
func retakeScheduleFields(r queries.Retake) changelog.Fields {
	f := changelog.Fields{
		"building":         r.Building,
		"room":             r.Room,
		"scheduled_at":     r.ScheduledAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		"duration_minutes": r.DurationMinutes,
	}
	if r.Notes != nil {
		f["notes"] = *r.Notes
	}
	return f
}

// fromRow конвертирует sqlc-структуру в доменную модель.
func fromRow(r queries.RetakeChangeRequest) (Request, error) {
	var changes Changes
	if err := json.Unmarshal(r.RequestedChanges, &changes); err != nil {
		return Request{}, fmt.Errorf("changerequest: unmarshal requested_changes: %w", err)
	}
	out := Request{
		ID:             pgutil.UUID(r.ID),
		RetakeID:       pgutil.UUID(r.RetakeID),
		RequestedBy:    pgutil.UUID(r.RequestedBy),
		Changes:        changes,
		Reason:         r.Reason,
		Status:         r.Status,
		DecisionReason: r.DecisionReason,
		CreatedAt:      r.CreatedAt.Time,
	}
	if r.ReviewedBy.Valid {
		id := pgutil.UUID(r.ReviewedBy)
		out.ReviewedBy = &id
	}
	if r.ReviewedAt.Valid {
		t := r.ReviewedAt.Time
		out.ReviewedAt = &t
	}
	if r.UpdatedAt.Valid {
		t := r.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out, nil
}

// fromRows конвертирует срез sqlc-структур.
func fromRows(rows []queries.RetakeChangeRequest) ([]Request, error) {
	out := make([]Request, 0, len(rows))
	for _, r := range rows {
		req, err := fromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, nil
}
