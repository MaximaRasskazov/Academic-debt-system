package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// mountSync регистрирует /api/sync/*:
//   - GET  /status  — время последней синхронизации (любой авторизованный)
//   - POST /trigger — запустить синхронизацию вручную (admin/dean)
//
// Если syncSvc nil (EMULATOR_URL не задан в .env) — handler сам вернёт
// 503 service_disabled, а не падёт с nil-разыменованием.
func mountSync(r chi.Router, d Deps) {
	h := handler.NewSyncHandler(d.Sync)
	r.Route("/api/sync", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))
		r.Get("/status", h.Status)
		r.With(mw.RequirePermission(d.RBAC, "roles.assign")).Post("/trigger", h.Trigger)
	})
}
