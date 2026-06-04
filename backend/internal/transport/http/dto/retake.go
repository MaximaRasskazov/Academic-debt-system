package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// RetakeResponse — публичное представление пересдачи.
type RetakeResponse struct {
	ID              uuid.UUID  `json:"id"`
	DisciplineID    uuid.UUID  `json:"discipline_id"`
	Kind            string     `json:"kind"` // regular / commission
	MinTeachers     int32      `json:"min_teachers"`
	Building        string     `json:"building"`
	Room            string     `json:"room"`
	ScheduledAt     time.Time  `json:"scheduled_at"`
	DurationMinutes int32      `json:"duration_minutes"`
	Status          string     `json:"status"` // scheduled / in_progress / completed / cancelled
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedBy       uuid.UUID  `json:"created_by"`
	Notes           *string    `json:"notes,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

// RetakeParticipantResponse — участник пересдачи.
type RetakeParticipantResponse struct {
	ID        uuid.UUID  `json:"id"`
	RetakeID  uuid.UUID  `json:"retake_id"`
	UserID    uuid.UUID  `json:"user_id"`
	Kind      string     `json:"kind"` // student / teacher / commission_member
	DebtID    *uuid.UUID `json:"debt_id,omitempty"`
	Grade     *int32     `json:"grade,omitempty"`
	GradedAt  *time.Time `json:"graded_at,omitempty"`
	GradedBy  *uuid.UUID `json:"graded_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateRetakeRequest — POST /api/retakes.
// kind допустимы только "regular" или "commission".
type CreateRetakeRequest struct {
	DisciplineID    uuid.UUID `json:"discipline_id"`
	Kind            string    `json:"kind"`
	Building        string    `json:"building"`
	Room            string    `json:"room"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int32     `json:"duration_minutes"`
	Notes           *string   `json:"notes,omitempty"`
}

// UpdateRetakeRequest — PATCH /api/retakes/:id. PATCH-семантика:
// поля nil не меняют значения.
type UpdateRetakeRequest struct {
	Building        *string    `json:"building,omitempty"`
	Room            *string    `json:"room,omitempty"`
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`
	DurationMinutes *int32     `json:"duration_minutes,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
}

// SetRetakeStatusRequest — POST /api/retakes/:id/status.
// Ручная установка статуса деканом: scheduled | in_progress | completed | cancelled.
type SetRetakeStatusRequest struct {
	Status string `json:"status"`
}

// AddRetakeStudentRequest — POST /api/retakes/:id/students.
type AddRetakeStudentRequest struct {
	StudentID uuid.UUID `json:"student_id"`
	DebtID    uuid.UUID `json:"debt_id"`
}

// AddRetakeTeacherRequest — POST /api/retakes/:id/teachers.
type AddRetakeTeacherRequest struct {
	TeacherID uuid.UUID `json:"teacher_id"`
}

// RetakesListResponse — список с пагинацией для деканата.
type RetakesListResponse struct {
	Items  []RetakeResponse `json:"items"`
	Limit  int32            `json:"limit"`
	Offset int32            `json:"offset"`
}

// FromRetake — sqlc-структура → response.
func FromRetake(r queries.Retake) RetakeResponse {
	out := RetakeResponse{
		ID:              pgutil.UUID(r.ID),
		DisciplineID:    pgutil.UUID(r.DisciplineID),
		Kind:            r.Kind,
		MinTeachers:     r.MinTeachers,
		Building:        r.Building,
		Room:            r.Room,
		ScheduledAt:     r.ScheduledAt.Time,
		DurationMinutes: r.DurationMinutes,
		Status:          r.Status,
		CreatedBy:       pgutil.UUID(r.CreatedBy),
		Notes:           r.Notes,
		CreatedAt:       r.CreatedAt.Time,
	}
	if r.CompletedAt.Valid {
		t := r.CompletedAt.Time
		out.CompletedAt = &t
	}
	if r.UpdatedAt.Valid {
		t := r.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out
}

// FromRetakes — срез.
func FromRetakes(rs []queries.Retake) []RetakeResponse {
	out := make([]RetakeResponse, 0, len(rs))
	for _, r := range rs {
		out = append(out, FromRetake(r))
	}
	return out
}

// FromRetakeParticipant — sqlc → response.
func FromRetakeParticipant(p queries.RetakeParticipant) RetakeParticipantResponse {
	out := RetakeParticipantResponse{
		ID:        pgutil.UUID(p.ID),
		RetakeID:  pgutil.UUID(p.RetakeID),
		UserID:    pgutil.UUID(p.UserID),
		Kind:      p.Kind,
		Grade:     p.Grade,
		CreatedAt: p.CreatedAt.Time,
	}
	if p.DebtID.Valid {
		id := pgutil.UUID(p.DebtID)
		out.DebtID = &id
	}
	if p.GradedAt.Valid {
		t := p.GradedAt.Time
		out.GradedAt = &t
	}
	if p.GradedBy.Valid {
		id := pgutil.UUID(p.GradedBy)
		out.GradedBy = &id
	}
	return out
}

// FromRetakeParticipants — срез.
func FromRetakeParticipants(ps []queries.RetakeParticipant) []RetakeParticipantResponse {
	out := make([]RetakeParticipantResponse, 0, len(ps))
	for _, p := range ps {
		out = append(out, FromRetakeParticipant(p))
	}
	return out
}
