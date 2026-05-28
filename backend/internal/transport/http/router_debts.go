package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountDebts регистрирует /api/debts/* с разделением по permissions:
//   - /my              → debts.view.own (студент)
//   - /by-discipline   → debts.view.by_discipline (преподаватель)
//   - /, /:id, /summary → debts.view.all (дeканат/админ)
//   - POST             → debts.create (преподаватель)
//   - PATCH /grade     → debts.update (преподаватель)
//   - PATCH /cancel    → debts.delete (деканат)
func mountDebts(r chi.Router, d Deps) {
	h := handler.NewDebtHandler(d.Debts)

	r.Route("/api/debts", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))

		r.With(mw.RequirePermission(d.RBAC, "debts.view.own")).
			Get("/my", h.ListMy)

		r.With(mw.RequirePermission(d.RBAC, "debts.view.by_discipline")).
			Get("/by-discipline", h.ListByDiscipline)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "debts.view.all"))
			r.Get("/", h.ListAll)
			r.Get("/summary", h.Summary)
		})

		// /:id — только debts.view.all. Студент/препод смотрят через свои списки.
		r.With(mw.RequirePermission(d.RBAC, "debts.view.all")).
			Get("/{id}", h.Get)

		r.With(mw.RequirePermission(d.RBAC, "debts.create")).
			Post("/", h.Create)

		r.With(mw.RequirePermission(d.RBAC, "debts.update")).
			Patch("/{id}/grade", h.Grade)

		r.With(mw.RequirePermission(d.RBAC, "debts.delete")).
			Patch("/{id}/cancel", h.Cancel)
	})
}
