package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changerequest"
)

// ChangeRequestResponse — публичное представление заявки на изменение.
type ChangeRequestResponse struct {
	ID             uuid.UUID             `json:"id"`
	RetakeID       uuid.UUID             `json:"retake_id"`
	RequestedBy    uuid.UUID             `json:"requested_by"`
	Changes        changerequest.Changes `json:"requested_changes"`
	Reason         *string               `json:"reason,omitempty"`
	Status         string                `json:"status"`
	ReviewedBy     *uuid.UUID            `json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time            `json:"reviewed_at,omitempty"`
	DecisionReason *string               `json:"decision_reason,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      *time.Time            `json:"updated_at,omitempty"`
}

// SubmitChangeRequestBody — POST /api/retake-change-requests.
type SubmitChangeRequestBody struct {
	RetakeID uuid.UUID             `json:"retake_id"`
	Changes  changerequest.Changes `json:"requested_changes"`
	Reason   *string               `json:"reason,omitempty"`
}

// DecisionBody — тело для /approve и /reject.
// DecisionReason обязателен для reject, опционален для approve.
type DecisionBody struct {
	DecisionReason *string `json:"decision_reason,omitempty"`
}

// ChangeRequestsListResponse — список с пагинацией для деканата.
type ChangeRequestsListResponse struct {
	Items  []ChangeRequestResponse `json:"items"`
	Limit  int32                   `json:"limit"`
	Offset int32                   `json:"offset"`
}

// FromChangeRequest конвертирует доменную модель в DTO.
func FromChangeRequest(r changerequest.Request) ChangeRequestResponse {
	return ChangeRequestResponse{
		ID:             r.ID,
		RetakeID:       r.RetakeID,
		RequestedBy:    r.RequestedBy,
		Changes:        r.Changes,
		Reason:         r.Reason,
		Status:         r.Status,
		ReviewedBy:     r.ReviewedBy,
		ReviewedAt:     r.ReviewedAt,
		DecisionReason: r.DecisionReason,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// FromChangeRequests конвертирует срез.
func FromChangeRequests(rs []changerequest.Request) []ChangeRequestResponse {
	out := make([]ChangeRequestResponse, 0, len(rs))
	for _, r := range rs {
		out = append(out, FromChangeRequest(r))
	}
	return out
}
