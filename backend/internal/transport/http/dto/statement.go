package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// StatementSheetResponse — статус ведомости пересдачи для UI.
type StatementSheetResponse struct {
	RetakeID   uuid.UUID  `json:"retake_id"`
	Status     string     `json:"status"` // open | closed
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	ClosedBy   *uuid.UUID `json:"closed_by,omitempty"`
	ReopenedAt *time.Time `json:"reopened_at,omitempty"`
	ReopenedBy *uuid.UUID `json:"reopened_by,omitempty"`
}

// SaveDraftGradeRequest — тело PATCH /api/retakes/{id}/sheet/grades/{user_id}.
type SaveDraftGradeRequest struct {
	Grade int32 `json:"grade"`
}

// FromStatementSheet маппит sqlc-структуру в ответ API.
func FromStatementSheet(s queries.StatementSheet) StatementSheetResponse {
	out := StatementSheetResponse{
		RetakeID: pgutil.UUID(s.RetakeID),
		Status:   s.Status,
	}
	if s.ClosedAt.Valid {
		t := s.ClosedAt.Time
		out.ClosedAt = &t
	}
	if s.ClosedBy.Valid {
		id := pgutil.UUID(s.ClosedBy)
		out.ClosedBy = &id
	}
	if s.ReopenedAt.Valid {
		t := s.ReopenedAt.Time
		out.ReopenedAt = &t
	}
	if s.ReopenedBy.Valid {
		id := pgutil.UUID(s.ReopenedBy)
		out.ReopenedBy = &id
	}
	return out
}
