package middleware

import (
	"net/http"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/rbac"
)

// RequirePermission возвращает middleware, который пропускает запрос
// только если у текущего пользователя есть permission с указанным
// slug. Должен вызываться ПОСЛЕ Auth — иначе UserID(ctx) вернёт false
// и middleware ответит 401.
//
// 403 — есть пользователь, но нет permission. 401 — пользователя нет
// в контексте (запрос не прошёл Auth). 500 — ошибка БД при проверке.
func RequirePermission(svc *rbac.Service, permissionSlug string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserID(r.Context())
			if !ok {
				writeUnauthorized(w, "not authenticated")
				return
			}

			has, err := svc.HasPermission(r.Context(), userID, permissionSlug)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal","message":"permission check failed"}`))
				return
			}
			if !has {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"forbidden","message":"insufficient permissions"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
