package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountStatements регистрирует ведомость пересдачи под уже-вложенным
// роутером /api/retakes (вызывается из mountRetakes). Пути относительные:
//
//   - GET  /{id}/sheet              — статус ведомости (препод-участник / декан)
//   - PATCH /{id}/sheet/grades/{user_id} — черновик оценки (assign_grade)
//   - POST /{id}/sheet/close        — фиксация ведомости (assign_grade)
//   - POST /{id}/sheet/reopen       — открыть закрытую (ТОЛЬКО декан, create)
//
// requireRetakeAccess переиспользуется из router_retakes.go: декан видит
// любую пересдачу, преподаватель — только если сам участник. Reopen
// под retakes.create (в сидах только у декана) — guard не нужен, декан
// не участник пересдачи.
func mountStatements(r chi.Router, d Deps) {
	h := handler.NewStatementHandler(d.Statements)

	r.With(
		mw.RequireAnyPermission(d.RBAC, "retakes.view.own", "retakes.view.all"),
		requireRetakeAccess(d),
	).Get("/{id}/sheet", h.GetSheet)

	r.Group(func(r chi.Router) {
		r.Use(mw.RequirePermission(d.RBAC, "retakes.assign_grade"))
		r.Use(requireRetakeAccess(d))
		r.Patch("/{id}/sheet/grades/{user_id}", h.SaveDraftGrade)
		r.Post("/{id}/sheet/close", h.CloseSheet)
	})

	r.With(mw.RequirePermission(d.RBAC, "retakes.create")).
		Post("/{id}/sheet/reopen", h.ReopenSheet)
}
