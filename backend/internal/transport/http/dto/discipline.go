package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// DisciplineResponse — публичное представление дисциплины. Мы не отдаём
// deleted_at / deleted_by — клиенту это шум, активные записи и так
// получает только справочник.
type DisciplineResponse struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Code        string     `json:"code"`
	Description *string    `json:"description,omitempty"`
	ExternalID  *string    `json:"external_id,omitempty"`
	Source      string     `json:"source"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// CreateDisciplineRequest — тело POST /api/disciplines.
type CreateDisciplineRequest struct {
	Name        string  `json:"name"`
	Code        string  `json:"code"`
	Description *string `json:"description,omitempty"`
	ExternalID  *string `json:"external_id,omitempty"`
	Source      string  `json:"source,omitempty"` // manual / sync (default manual)
}

// UpdateDisciplineRequest — тело PATCH /api/disciplines/:id.
// nil-значения не меняют поля (PATCH-семантика).
type UpdateDisciplineRequest struct {
	Name        *string `json:"name,omitempty"`
	Code        *string `json:"code,omitempty"`
	Description *string `json:"description,omitempty"`
}

// AttachTeacherRequest — тело POST /api/disciplines/:id/teachers.
type AttachTeacherRequest struct {
	TeacherID uuid.UUID `json:"teacher_id"`
}

// AttachStudentRequest — тело POST /api/disciplines/:id/students.
type AttachStudentRequest struct {
	StudentID    uuid.UUID `json:"student_id"`
	AcademicYear *string   `json:"academic_year,omitempty"` // "2025-2026"
	Semester     *int32    `json:"semester,omitempty"`      // 1 или 2
	Source       string    `json:"source,omitempty"`        // manual / sync
}

// DisciplinesListResponse — пагинированный ответ.
type DisciplinesListResponse struct {
	Items  []DisciplineResponse `json:"items"`
	Total  int64                `json:"total"`
	Limit  int32                `json:"limit"`
	Offset int32                `json:"offset"`
}

// FromDiscipline маппит sqlc-структуру в публичный response.
func FromDiscipline(d queries.Discipline) DisciplineResponse {
	out := DisciplineResponse{
		ID:          pgutil.UUID(d.ID),
		Name:        d.Name,
		Code:        d.Code,
		Description: d.Description,
		ExternalID:  d.ExternalID,
		Source:      d.Source,
		CreatedAt:   d.CreatedAt.Time,
	}
	if d.UpdatedAt.Valid {
		t := d.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out
}

// FromDisciplines — srez → []response.
func FromDisciplines(ds []queries.Discipline) []DisciplineResponse {
	out := make([]DisciplineResponse, 0, len(ds))
	for _, d := range ds {
		out = append(out, FromDiscipline(d))
	}
	return out
}
