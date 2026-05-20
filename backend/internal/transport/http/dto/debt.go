package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// DebtResponse — публичное представление долга. deleted_at / deleted_by
// не отдаём — фронт работает только с активными записями.
type DebtResponse struct {
	ID           uuid.UUID  `json:"id"`
	StudentID    uuid.UUID  `json:"student_id"`
	DisciplineID uuid.UUID  `json:"discipline_id"`
	IssuedBy     uuid.UUID  `json:"issued_by"`
	Status       string     `json:"status"` // open / graded / cancelled
	FinalGrade   *int32     `json:"final_grade,omitempty"`
	GradedAt     *time.Time `json:"graded_at,omitempty"`
	GradedBy     *uuid.UUID `json:"graded_by,omitempty"`
	Source       string     `json:"source"`
	ExternalID   *string    `json:"external_id,omitempty"`
	Notes        *string    `json:"notes,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

// CreateDebtRequest — POST /api/debts.
type CreateDebtRequest struct {
	StudentID    uuid.UUID `json:"student_id"`
	DisciplineID uuid.UUID `json:"discipline_id"`
	ExternalID   *string   `json:"external_id,omitempty"`
	Source       string    `json:"source,omitempty"`
	Notes        *string   `json:"notes,omitempty"`
}

// GradeDebtRequest — PATCH /api/debts/:id/grade.
type GradeDebtRequest struct {
	Grade int32 `json:"grade"`
}

// DebtsListResponse — пагинированный ответ.
type DebtsListResponse struct {
	Items  []DebtResponse `json:"items"`
	Total  int64          `json:"total"`
	Limit  int32          `json:"limit"`
	Offset int32          `json:"offset"`
}

// DebtSummaryRow — строка сводной таблицы для деканата.
type DebtSummaryRow struct {
	DisciplineID uuid.UUID `json:"discipline_id"`
	OpenCount    int64     `json:"open_count"`
	GradedCount  int64     `json:"graded_count"`
}

// FromDebt маппит sqlc-структуру в response.
func FromDebt(d queries.Debt) DebtResponse {
	out := DebtResponse{
		ID:           pgutil.UUID(d.ID),
		StudentID:    pgutil.UUID(d.StudentID),
		DisciplineID: pgutil.UUID(d.DisciplineID),
		IssuedBy:     pgutil.UUID(d.IssuedBy),
		Status:       d.Status,
		FinalGrade:   d.FinalGrade,
		Source:       d.Source,
		ExternalID:   d.ExternalID,
		Notes:        d.Notes,
		CreatedAt:    d.CreatedAt.Time,
	}
	if d.GradedAt.Valid {
		t := d.GradedAt.Time
		out.GradedAt = &t
	}
	if d.GradedBy.Valid {
		id := pgutil.UUID(d.GradedBy)
		out.GradedBy = &id
	}
	if d.UpdatedAt.Valid {
		t := d.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out
}

// FromDebts превращает срез sqlc → срез response.
func FromDebts(ds []queries.Debt) []DebtResponse {
	out := make([]DebtResponse, 0, len(ds))
	for _, d := range ds {
		out = append(out, FromDebt(d))
	}
	return out
}

// FromDebtSummary маппит строку сводной выборки.
func FromDebtSummary(r queries.SummaryDebtsByDisciplineRow) DebtSummaryRow {
	return DebtSummaryRow{
		DisciplineID: pgutil.UUID(r.DisciplineID),
		OpenCount:    r.OpenCount,
		GradedCount:  r.GradedCount,
	}
}

// FromDebtSummaries — срез строк.
func FromDebtSummaries(rs []queries.SummaryDebtsByDisciplineRow) []DebtSummaryRow {
	out := make([]DebtSummaryRow, 0, len(rs))
	for _, r := range rs {
		out = append(out, FromDebtSummary(r))
	}
	return out
}
