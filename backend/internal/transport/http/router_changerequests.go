package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountChangeRequests регистрирует /api/retake-change-requests/*.
// Преподаватель-участник подаёт заявку (retakes.request_change),
// деканат рассматривает (retakes.approve_change).
func mountChangeRequests(r chi.Router, d Deps) {
	h := handler.NewChangeRequestHandler(d.ChangeRequests)
	r.Route("/api/retake-change-requests", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))

		r.With(mw.RequirePermission(d.RBAC, "retakes.request_change")).
			Post("/", h.Submit)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "retakes.approve_change"))
			r.Get("/", h.ListPending)
			r.Post("/{id}/approve", h.Approve)
			r.Post("/{id}/reject", h.Reject)
		})
	})
}
