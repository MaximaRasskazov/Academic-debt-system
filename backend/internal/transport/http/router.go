// Package http собирает chi-роутер приложения: middleware-стек,
// /health и маршруты /api/auth/*. Сюда же в будущем подключатся
// доменные роуты (debts, retakes, ...) — каждый своим Sub-router'ом.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/config"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/auth"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/token"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// Deps — зависимости, которые роутер получает извне.
// Сделано отдельной структурой, чтобы main.go не разрастался
// сигнатурой NewRouter(a, b, c, d, ...).
type Deps struct {
	Cfg    *config.Config
	Pool   *pgxpool.Pool
	Auth   *auth.Service
	Tokens *token.Service
}

// NewRouter собирает chi-роутер: middleware → /health → /api/auth/*.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(mw.SecurityHeaders)
	r.Use(mw.CORS(d.Cfg.AllowedOrigins))

	r.Get("/health", healthHandler(d.Pool))

	authH := handler.NewAuthHandler(d.Auth, d.Cfg)
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", authH.Register)
		r.Post("/login", authH.Login)
		r.Post("/refresh", authH.Refresh)

		// Защищённые endpoint'ы используют Auth-middleware.
		r.Group(func(r chi.Router) {
			r.Use(mw.Auth(d.Tokens))
			r.Post("/logout", authH.Logout)
			r.Get("/me", authH.Me)
		})
	})

	return r
}

// healthHandler — копия логики из cmd/server/main.go, вынесенная в
// один пакет с роутером. Возвращает 200 если pool.Ping проходит,
// иначе 503. Используется healthcheck'ом docker-compose.
func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "db_unreachable"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
