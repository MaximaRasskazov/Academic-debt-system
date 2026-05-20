package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/report"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
)

// ReportHandler обслуживает /api/reports/*.
type ReportHandler struct {
	svc *report.Service
}

func NewReportHandler(svc *report.Service) *ReportHandler {
	return &ReportHandler{svc: svc}
}

// DebtsSummary — GET /api/reports/debts-summary.
// Возвращает сводку по дисциплинам (open/graded count). Permission
// reports.export проверяется на маршруте, здесь handler уже знает,
// что вызывающий имеет право.
func (h *ReportHandler) DebtsSummary(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.DebtsSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось собрать сводку")
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDebtsSummary(rows))
}

// Retakes — GET /api/reports/retakes?from=2026-01-01&to=2026-07-01[&format=xlsx|csv]
//
// Без format — возвращает JSON. С format=xlsx|csv — бинарный поток
// с правильным Content-Type и Content-Disposition (для скачивания).
//
// Период — даты в формате YYYY-MM-DD (включительно from, не включая to).
// Если даты не указаны — берётся последний месяц.
func (h *ReportHandler) Retakes(w http.ResponseWriter, r *http.Request) {
	from, to, err := parsePeriod(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_period", err.Error())
		return
	}

	rows, err := h.svc.RetakesForPeriod(r.Context(), from, to)
	if err != nil {
		if errors.Is(err, report.ErrInvalidPeriod) {
			writeError(w, http.StatusBadRequest, "invalid_period", "from должен быть раньше to")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "не удалось собрать отчёт")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		// JSON по умолчанию — удобно для разработки и предпросмотра.
		writeJSON(w, http.StatusOK, dto.FromRetakesReport(rows))
		return
	}

	exportFormat := report.ExportFormat(format)
	body, err := report.ExportRetakes(rows, exportFormat)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unsupported_format", "формат должен быть xlsx или csv")
		return
	}

	filename := fmt.Sprintf("retakes-%s_%s.%s",
		from.Format("2006-01-02"), to.Format("2006-01-02"), exportFormat.FileExt())
	w.Header().Set("Content-Type", exportFormat.ContentType())
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	// gosec G705 false positive: body — это XLSX или CSV, не HTML.
	// Content-Type выставлен явно (spreadsheetml / text/csv), браузер
	// не будет интерпретировать содержимое как HTML — XSS невозможен.
	_, _ = w.Write(body) //nolint:gosec // G705: бинарный отчёт, не HTML
}

// parsePeriod парсит query-параметры from/to. Пустые значения берут
// дефолт "последний месяц".
//
// Допустимый формат — YYYY-MM-DD (date) или RFC3339 (полный datetime).
// Возвращаемые time.Time — в UTC, что соответствует хранению в БД.
func parsePeriod(fromStr, toStr string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	defaultTo := now.AddDate(0, 0, 1)          // завтра, чтобы today входил
	defaultFrom := defaultTo.AddDate(0, -1, 0) // месяц назад

	from := defaultFrom
	to := defaultTo

	if fromStr != "" {
		t, err := parseDateOrDateTime(fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("from: %w", err)
		}
		from = t
	}
	if toStr != "" {
		t, err := parseDateOrDateTime(toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("to: %w", err)
		}
		to = t
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("from должен быть раньше to")
	}
	return from, to, nil
}

func parseDateOrDateTime(s string) (time.Time, error) {
	// Сначала пробуем дату.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	// Полный RFC3339.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("ожидается YYYY-MM-DD или RFC3339, получено %q", s)
}
