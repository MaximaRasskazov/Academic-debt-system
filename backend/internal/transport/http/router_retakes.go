package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountRetakes регистрирует /api/retakes/*:
//   - /my              → retakes.view.own (студент / препод)
//   - / и /:id и т.п.  → retakes.view.all (деканат, админ)
//   - POST             → retakes.create (деканат)
//   - PATCH /:id       → retakes.update (деканат)
//   - lifecycle (start/complete/cancel) → retakes.update
//   - участники + grade → retakes.update / retakes.assign_grade
func mountRetakes(r chi.Router, d Deps) {
	h := handler.NewRetakeHandler(d.Retakes)

	r.Route("/api/retakes", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))

		r.With(mw.RequirePermission(d.RBAC, "retakes.view.own")).
			Get("/my", h.ListMy)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "retakes.view.all"))
			r.Get("/", h.ListAll)
			r.Get("/{id}", h.Get)
			r.Get("/{id}/participants", h.ListParticipants)
		})

		r.With(mw.RequirePermission(d.RBAC, "retakes.create")).
			Post("/", h.Create)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "retakes.update"))
			r.Patch("/{id}", h.Update)
			r.Post("/{id}/start", h.Start)
			r.Post("/{id}/complete", h.Complete)
			r.Post("/{id}/cancel", h.Cancel)
			r.Post("/{id}/students", h.AddStudent)
			r.Delete("/{id}/students/{user_id}", h.RemoveStudent)
			r.Post("/{id}/teachers", h.AddTeacher)
			r.Delete("/{id}/teachers/{user_id}", h.RemoveTeacher)
		})

		r.With(mw.RequirePermission(d.RBAC, "retakes.assign_grade")).
			Patch("/{id}/students/{user_id}/grade", h.GradeStudent)
	})
}
