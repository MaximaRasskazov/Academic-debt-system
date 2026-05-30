package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountRetakeRequests регистрирует /api/retake-requests/*.
//
// Преподаватель (permission retakes.request) подаёт заявку и видит свои.
// Деканат (permission retakes.create) видит pending-заявки и одобряет/
// отклоняет. retakes.create — то же право, что нужно для прямого
// создания пересдачи через /api/retakes, что логично: approve по сути
// и есть создание пересдачи руками декана.
func mountRetakeRequests(r chi.Router, d Deps) {
	h := handler.NewRetakeRequestHandler(d.RetakeRequests)
	r.Route("/api/retake-requests", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))

		// Преподаватель.
		r.With(mw.RequirePermission(d.RBAC, "retakes.request")).
			Post("/", h.Submit)
		r.With(mw.RequirePermission(d.RBAC, "retakes.request")).
			Get("/my", h.ListMy)

		// Деканат.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "retakes.create"))
			r.Get("/", h.ListPending)
			r.Post("/{id}/approve", h.Approve)
			r.Post("/{id}/reject", h.Reject)
		})
	})
}
