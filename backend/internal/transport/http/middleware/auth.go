// Package middleware содержит HTTP-middleware приложения: auth,
// RBAC-проверки, CORS и заголовки безопасности. Используется в
// internal/transport/http/router при сборке chi-роутера.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/token"
)

// ctxKey — приватный тип ключа в request-контексте, чтобы значения,
// положенные этим middleware, не пересекались по имени с чужими.
type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyTokenID
)

// Auth возвращает middleware, валидирующее Bearer-токен через
// TokenService. При успехе кладёт в контекст userID и tokenID и
// пропускает запрос дальше. На любую ошибку отвечает 401 — мы не
// различаем "expired", "revoked" и "invalid" наружу, чтобы не давать
// атакующему лишних подсказок.
func Auth(tokens *token.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := extractBearer(r.Header.Get("Authorization"))
			if !ok {
				writeUnauthorized(w, "missing or malformed Authorization header")
				return
			}

			userID, tokenID, err := tokens.Validate(r.Context(), raw)
			if err != nil {
				switch {
				case errors.Is(err, token.ErrTokenExpired):
					writeUnauthorized(w, "token expired")
				case errors.Is(err, token.ErrTokenRevoked):
					writeUnauthorized(w, "token revoked")
				default:
					writeUnauthorized(w, "invalid token")
				}
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
			ctx = context.WithValue(ctx, ctxKeyTokenID, tokenID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID достаёт userID, положенный Auth-middleware. Если запрос не
// прошёл через Auth — вернёт uuid.Nil и false. Handler'ы должны
// проверять ok и отвечать 500, если попали без Auth (это баг сборки
// роутера, а не легитимный кейс).
func UserID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return v, ok
}

// TokenID достаёт ID access-токена (он же jti), которым клиент
// аутентифицирован. Нужен для logout — точечная ревокация текущей
// сессии.
func TokenID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxKeyTokenID).(uuid.UUID)
	return v, ok
}

// extractBearer выделяет токен из заголовка вида "Bearer <jwt>".
// Любое отклонение от формата считаем "нет токена" — без подсказок
// клиенту, что именно не так.
func extractBearer(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(header[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}

func writeUnauthorized(w http.ResponseWriter, reason string) {
	// Заголовок WWW-Authenticate сообщает клиенту, что нужен Bearer-токен.
	// Полезно для отладки на фронте.
	w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","message":"` + reason + `"}`))
}
