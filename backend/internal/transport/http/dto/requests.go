package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changerequest"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/teacherrequest"
)

// TeacherRequestResponse — DTO для заявки на роль преподавателя.
type TeacherRequestResponse struct {
	ID             uuid.UUID  `json:"id"`
	RequestedBy    uuid.UUID  `json:"requested_by"`
	Reason         *string    `json:"reason,omitempty"`
	Status         string     `json:"status"`
	ReviewedBy     *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	DecisionReason *string    `json:"decision_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	// Данные пользователя-заявителя.
	UserEmail      string  `json:"user_email"`
	UserFirstName  string  `json:"user_first_name"`
	UserLastName   string  `json:"user_last_name"`
	UserMiddleName *string `json:"user_middle_name,omitempty"`
}

// RejectRequestBody — тело для POST .../reject.
type RejectRequestBody struct {
	Reason string `json:"reason"`
}

// FromTeacherRequest конвертирует WithUser → TeacherRequestResponse.
func FromTeacherRequest(r teacherrequest.WithUser) TeacherRequestResponse {
	out := TeacherRequestResponse{
		ID:             pgutil.UUID(r.ID),
		RequestedBy:    pgutil.UUID(r.RequestedBy),
		Reason:         r.Reason,
		Status:         r.Status,
		CreatedAt:      r.CreatedAt.Time,
		UserEmail:      r.Email,
		UserFirstName:  r.FirstName,
		UserLastName:   r.LastName,
		UserMiddleName: r.MiddleName,
	}
	if r.ReviewedBy.Valid {
		id := pgutil.UUID(r.ReviewedBy)
		out.ReviewedBy = &id
	}
	if r.ReviewedAt.Valid {
		t := r.ReviewedAt.Time
		out.ReviewedAt = &t
	}
	out.DecisionReason = r.DecisionReason
	return out
}

// FromTeacherRequests конвертирует срез.
func FromTeacherRequests(rs []teacherrequest.WithUser) []TeacherRequestResponse {
	out := make([]TeacherRequestResponse, 0, len(rs))
	for _, r := range rs {
		out = append(out, FromTeacherRequest(r))
	}
	return out
}

// TeacherRequestsListResponse — список с пагинацией.
type TeacherRequestsListResponse struct {
	Items  []TeacherRequestResponse `json:"items"`
	Limit  int32                    `json:"limit"`
	Offset int32                    `json:"offset"`
}

// ChangeRequestResponse — DTO для заявки на изменение пересдачи.
type ChangeRequestResponse struct {
	ID               uuid.UUID       `json:"id"`
	RetakeID         uuid.UUID       `json:"retake_id"`
	RequestedBy      uuid.UUID       `json:"requested_by"`
	RequestedChanges json.RawMessage `json:"requested_changes"`
	Reason           *string         `json:"reason,omitempty"`
	Status           string          `json:"status"`
	ReviewedBy       *uuid.UUID      `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time      `json:"reviewed_at,omitempty"`
	DecisionReason   *string         `json:"decision_reason,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	// Данные пользователя-заявителя.
	RequesterEmail      string  `json:"requester_email"`
	RequesterFirstName  string  `json:"requester_first_name"`
	RequesterLastName   string  `json:"requester_last_name"`
	RequesterMiddleName *string `json:"requester_middle_name,omitempty"`
}

// FromChangeRequest конвертирует WithUser → ChangeRequestResponse.
func FromChangeRequest(r changerequest.WithUser) ChangeRequestResponse {
	out := ChangeRequestResponse{
		ID:                  pgutil.UUID(r.ID),
		RetakeID:            pgutil.UUID(r.RetakeID),
		RequestedBy:         pgutil.UUID(r.RequestedBy),
		RequestedChanges:    json.RawMessage(r.RequestedChanges),
		Reason:              r.Reason,
		Status:              r.Status,
		CreatedAt:           r.CreatedAt.Time,
		RequesterEmail:      r.RequesterEmail,
		RequesterFirstName:  r.RequesterFirstName,
		RequesterLastName:   r.RequesterLastName,
		RequesterMiddleName: r.RequesterMiddleName,
	}
	if r.ReviewedBy.Valid {
		id := pgutil.UUID(r.ReviewedBy)
		out.ReviewedBy = &id
	}
	if r.ReviewedAt.Valid {
		t := r.ReviewedAt.Time
		out.ReviewedAt = &t
	}
	out.DecisionReason = r.DecisionReason
	return out
}

// FromChangeRequests конвертирует срез.
func FromChangeRequests(rs []changerequest.WithUser) []ChangeRequestResponse {
	out := make([]ChangeRequestResponse, 0, len(rs))
	for _, r := range rs {
		out = append(out, FromChangeRequest(r))
	}
	return out
}

// ChangeRequestsListResponse — список с пагинацией.
type ChangeRequestsListResponse struct {
	Items  []ChangeRequestResponse `json:"items"`
	Limit  int32                   `json:"limit"`
	Offset int32                   `json:"offset"`
}
