package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
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
		})

		// Состав участников: деканат (view.all) видит любую пересдачу,
		// студент/преподаватель (view.own) — только если сам участник этой
		// пересдачи. Без этого преподаватель не может построить ведомость
		// своей пересдачи (право grade у него есть, а увидеть кого
		// грейдить — не было).
		r.With(
			mw.RequireAnyPermission(d.RBAC, "retakes.view.own", "retakes.view.all"),
			requireRetakeAccess(d),
		).Get("/{id}/participants", h.ListParticipants)

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

// requireRetakeAccess пропускает запрос, если у пользователя есть
// retakes.view.all (деканат/админ — видят любую пересдачу) ИЛИ он сам
// участник запрашиваемой пересдачи. Не даёт одному преподавателю
// смотреть состав чужой пересдачи, оставляя доступ к своим — ровно то,
// что нужно для ведомости.
func requireRetakeAccess(d Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := mw.UserID(r.Context())
			if !ok {
				writeRouterError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
				return
			}

			if has, err := d.RBAC.HasPermission(r.Context(), userID, "retakes.view.all"); err != nil {
				writeRouterError(w, http.StatusInternalServerError, "internal", "permission check failed")
				return
			} else if has {
				next.ServeHTTP(w, r)
				return
			}

			retakeID, err := uuid.Parse(chi.URLParam(r, "id"))
			if err != nil {
				writeRouterError(w, http.StatusBadRequest, "invalid_id", "некорректный id пересдачи")
				return
			}

			parts, err := d.Retakes.ListParticipants(r.Context(), retakeID)
			if err != nil {
				writeRouterError(w, http.StatusInternalServerError, "internal", "не удалось проверить доступ")
				return
			}
			for _, p := range parts {
				if pgutil.UUID(p.UserID) == userID {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeRouterError(w, http.StatusForbidden, "forbidden", "вы не участник этой пересдачи")
		})
	}
}

// writeRouterError — минимальный JSON-ответ об ошибке для middleware
// этого пакета (хелпер writeError живёт в пакете handler и сюда не
// импортируется во избежание цикла).
func writeRouterError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + code + `","message":"` + msg + `"}`))
}
