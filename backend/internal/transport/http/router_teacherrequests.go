package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountTeacherRequests регистрирует /api/teacher-requests/*:
//   - POST /            — любой авторизованный (подать заявку)
//   - GET  /my          — свои заявки (любой авторизованный)
//   - GET  /            — pending-список (деканат, teacher_request.review)
//   - POST /:id/approve — одобрить (деканат)
//   - POST /:id/reject  — отклонить (деканат)
func mountTeacherRequests(r chi.Router, d Deps) {
	h := handler.NewTeacherRequestHandler(d.TeacherRequests)

	r.Route("/api/teacher-requests", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))

		r.Post("/", h.Create)
		r.Get("/my", h.ListMy)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "teacher_request.review"))
			r.Get("/", h.ListPending)
			r.Post("/{id}/approve", h.Approve)
			r.Post("/{id}/reject", h.Reject)
		})
	})
}
