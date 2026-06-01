package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retakerequest"
)

// SubmitRetakeRequestBody — POST /api/retake-requests.
// Преподаватель описывает желаемую пересдачу: дату, место, состав.
//
// student_debt_ids — uuid конкретных открытых долгов, которые должны
// закрыться на этой пересдаче. Это симметрично API
// POST /api/retakes/{id}/students (тот тоже принимает debt_id).
type SubmitRetakeRequestBody struct {
	DisciplineID    uuid.UUID   `json:"discipline_id"`
	Kind            string      `json:"kind"` // "regular" / "commission"
	ScheduledAt     time.Time   `json:"scheduled_at"`
	DurationMinutes int32       `json:"duration_minutes"`
	Building        string      `json:"building"`
	Room            string      `json:"room"`
	Notes           *string     `json:"notes,omitempty"`
	Reason          *string     `json:"reason,omitempty"`
	StudentDebtIDs  []uuid.UUID `json:"student_debt_ids,omitempty"`
	TeacherIDs      []uuid.UUID `json:"teacher_ids,omitempty"`
}

// RetakeRequestResponse — публичное представление заявки на создание пересдачи.
type RetakeRequestResponse struct {
	ID              uuid.UUID             `json:"id"`
	RequestedBy     uuid.UUID             `json:"requested_by"`
	Payload         retakerequest.Payload `json:"payload"`
	Status          string                `json:"status"`
	ReviewedBy      *uuid.UUID            `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time            `json:"reviewed_at,omitempty"`
	DecisionReason  *string               `json:"decision_reason,omitempty"`
	CreatedRetakeID *uuid.UUID            `json:"created_retake_id,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       *time.Time            `json:"updated_at,omitempty"`
}

// RetakeRequestsListResponse — список с пагинацией.
type RetakeRequestsListResponse struct {
	Items  []RetakeRequestResponse `json:"items"`
	Limit  int32                   `json:"limit"`
	Offset int32                   `json:"offset"`
}

// FromRetakeRequest — конверсия доменной модели в DTO.
func FromRetakeRequest(r retakerequest.Request) RetakeRequestResponse {
	return RetakeRequestResponse{
		ID:              r.ID,
		RequestedBy:     r.RequestedBy,
		Payload:         r.Payload,
		Status:          r.Status,
		ReviewedBy:      r.ReviewedBy,
		ReviewedAt:      r.ReviewedAt,
		DecisionReason:  r.DecisionReason,
		CreatedRetakeID: r.CreatedRetakeID,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

// FromRetakeRequests — конверсия среза.
func FromRetakeRequests(rs []retakerequest.Request) []RetakeRequestResponse {
	out := make([]RetakeRequestResponse, 0, len(rs))
	for _, r := range rs {
		out = append(out, FromRetakeRequest(r))
	}
	return out
}
