package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountReports регистрирует /api/reports/*. Все эндпоинты требуют
// permission reports.export — он есть только у dean и admin из сидов.
func mountReports(r chi.Router, d Deps) {
	h := handler.NewReportHandler(d.Reports)
	r.Route("/api/reports", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))
		r.Use(mw.RequirePermission(d.RBAC, "reports.export"))
		r.Get("/debts-summary", h.DebtsSummary)
		r.Get("/retakes", h.Retakes)
	})
}
