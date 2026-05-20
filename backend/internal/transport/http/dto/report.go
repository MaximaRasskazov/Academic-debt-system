package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/report"
)

// DebtsSummaryRow — строка ответа GET /api/reports/debts-summary.
type DebtsSummaryRow struct {
	DisciplineID   uuid.UUID `json:"discipline_id"`
	DisciplineName string    `json:"discipline_name"`
	DisciplineCode string    `json:"discipline_code"`
	OpenCount      int64     `json:"open_count"`
	GradedCount    int64     `json:"graded_count"`
}

// RetakesReportRow — JSON-представление одной строки отчёта по
// пересдачам. Возвращается, если в запросе НЕ указан ?format=xlsx|csv
// (тогда отчёт идёт бинарником, см. ContentType ExportFormat).
type RetakesReportRow struct {
	RetakeID        uuid.UUID           `json:"retake_id"`
	DisciplineName  string              `json:"discipline_name"`
	DisciplineCode  string              `json:"discipline_code"`
	Kind            string              `json:"kind"`
	Building        string              `json:"building"`
	Room            string              `json:"room"`
	ScheduledAt     time.Time           `json:"scheduled_at"`
	CompletedAt     time.Time           `json:"completed_at"`
	DurationMinutes int32               `json:"duration_minutes"`
	Students        []ReportParticipant `json:"students"`
	Teachers        []ReportParticipant `json:"teachers"`
}

// ReportParticipant — участник пересдачи в отчёте.
type ReportParticipant struct {
	UserID    uuid.UUID `json:"user_id"`
	FullName  string    `json:"full_name"`
	GroupName *string   `json:"group_name,omitempty"`
	Email     string    `json:"email,omitempty"`
	Grade     *int32    `json:"grade,omitempty"`
}

// FromDebtsSummary — маппинг сервисных типов в DTO.
func FromDebtsSummary(rows []report.DisciplineSummary) []DebtsSummaryRow {
	out := make([]DebtsSummaryRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, DebtsSummaryRow{
			DisciplineID:   r.DisciplineID,
			DisciplineName: r.DisciplineName,
			DisciplineCode: r.DisciplineCode,
			OpenCount:      r.OpenCount,
			GradedCount:    r.GradedCount,
		})
	}
	return out
}

// FromRetakesReport — sqlc-собранный отчёт → response-формат.
func FromRetakesReport(rows []report.RetakeReportRow) []RetakesReportRow {
	out := make([]RetakesReportRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, RetakesReportRow{
			RetakeID:        r.RetakeID,
			DisciplineName:  r.DisciplineName,
			DisciplineCode:  r.DisciplineCode,
			Kind:            r.Kind,
			Building:        r.Building,
			Room:            r.Room,
			ScheduledAt:     r.ScheduledAt,
			CompletedAt:     r.CompletedAt,
			DurationMinutes: r.DurationMinutes,
			Students:        fromReportParticipants(r.Students),
			Teachers:        fromReportParticipants(r.Teachers),
		})
	}
	return out
}

func fromReportParticipants(ps []report.ParticipantInfo) []ReportParticipant {
	out := make([]ReportParticipant, 0, len(ps))
	for _, p := range ps {
		out = append(out, ReportParticipant{
			UserID:    p.UserID,
			FullName:  p.FullName,
			GroupName: p.GroupName,
			Email:     p.Email,
			Grade:     p.Grade,
		})
	}
	return out
}
