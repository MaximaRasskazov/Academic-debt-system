package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountDisciplines регистрирует /api/disciplines/* с RBAC-защитой:
//   - GET-эндпоинты: любой авторизованный (без отдельного permission)
//   - POST/PATCH/DELETE: соответствующие disciplines.create/update/delete
//
// Привязки teacher_disciplines/student_disciplines идут через disciplines.update —
// эти действия в курсаче делает деканат, отдельный permission заводить
// смысла нет.
func mountDisciplines(r chi.Router, d Deps) {
	h := handler.NewDisciplineHandler(d.Disciplines)

	r.Route("/api/disciplines", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))

		// Просмотр справочника — публичная информация для авторизованных.
		// В сидах disciplines.view выдан только teacher/dean/admin (это
		// отдельная permission под "управление справочником"), а GET-список
		// доступен всем — студент должен видеть состав дисциплин.
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.Get("/{id}/teachers", h.ListTeachers)
		r.Get("/{id}/students", h.ListStudents)

		r.With(mw.RequirePermission(d.RBAC, "disciplines.create")).
			Post("/", h.Create)

		r.With(mw.RequirePermission(d.RBAC, "disciplines.update")).
			Patch("/{id}", h.Update)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "disciplines.delete"))
			r.Delete("/{id}", h.Delete)
			r.Post("/{id}/restore", h.Restore)
		})

		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "disciplines.update"))
			r.Post("/{id}/teachers", h.AttachTeacher)
			r.Delete("/{id}/teachers/{user_id}", h.DetachTeacher)
			r.Post("/{id}/students", h.AttachStudent)
			r.Delete("/{id}/students/{user_id}", h.DetachStudent)
		})
	})
}

// mountMeDisciplines регистрирует /api/me/disciplines/* и
// /api/users/:id/disciplines/* — закрывает DoD BACK-01.
func mountMeDisciplines(r chi.Router, d Deps) {
	h := handler.NewUserDisciplinesHandler(d.Disciplines)

	r.Route("/api/me/disciplines", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))
		r.Get("/student", h.MyAsStudent)
		r.Get("/teacher", h.MyAsTeacher)
	})

	// Просмотр чужих привязок — только для тех, кто имеет users.view
	// (admin/dean из сидов). Это закрывает кейс "деканат смотрит,
	// кого преподаёт конкретный преподаватель".
	r.Route("/api/users/{id}/disciplines", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))
		r.Use(mw.RequirePermission(d.RBAC, "users.view"))
		r.Get("/student", h.UserAsStudent)
		r.Get("/teacher", h.UserAsTeacher)
	})
}
