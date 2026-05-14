package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/cors"
)

// CORS возвращает middleware с настройками под фронт-разработку.
// AllowCredentials=true нужен, чтобы браузер слал httpOnly-cookie
// с refresh-токеном на /api/auth/refresh.
//
// allowedOrigins берётся из config.Config.AllowedOrigins. Wildcard "*"
// несовместим с AllowCredentials=true, поэтому передаём явный список.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           int((5 * time.Minute).Seconds()),
	})
}
