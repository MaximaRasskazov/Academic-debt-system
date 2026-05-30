package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountDirectory — узкие справочники для форм UI.
//
//   - /api/teachers — список преподавателей (для составления комиссии
//     или приглашения в пересдачу). Доступен под retakes.request
//     (преподаватель, подающий заявку) или retakes.create (декан).
//
//   - /api/students/debtors?discipline_id=... — должники по дисциплине
//     с ФИО+группой. Доступен под debts.view.by_discipline (преподаватель)
//     или debts.view.all (декан).
//
// permission-границы здесь намеренно отличаются от /api/users и
// /api/debts: эти эндпойнты узкие (только ФИО, без email/birthday)
// и предназначены чисто для UI выбора. Учитель не должен ради
// заполнения формы получать users.view.
func mountDirectory(r chi.Router, d Deps) {
	h := handler.NewDirectoryHandler(d.Users, d.Debts)

	r.Route("/api/teachers", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))
		r.Use(mw.RequireAnyPermission(d.RBAC, "retakes.request", "retakes.create"))
		r.Get("/", h.ListTeachers)
	})

	r.Route("/api/students/debtors", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))
		r.Use(mw.RequireAnyPermission(d.RBAC, "debts.view.by_discipline", "debts.view.all"))
		r.Get("/", h.ListDebtors)
	})
}
