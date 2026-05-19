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
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/rbac"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/token"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// Deps — зависимости, которые роутер получает извне.
// Сделано отдельной структурой, чтобы main.go не разрастался
// сигнатурой NewRouter(a, b, c, d, ...).
type Deps struct {
	Cfg         *config.Config
	Pool        *pgxpool.Pool
	Auth        *auth.Service
	Tokens      *token.Service
	RBAC        *rbac.Service
	Disciplines *discipline.Service
	Notify      *notify.Service
	NotifyHub   *notify.Hub
}

// NewRouter собирает chi-роутер: middleware → /health → /api/*.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(mw.SecurityHeaders)
	r.Use(mw.CORS(d.Cfg.AllowedOrigins))

	r.Get("/health", healthHandler(d.Pool))

	// Обычные HTTP-маршруты — с таймаутом на запрос.
	// WebSocket вынесен отдельно: chimw.Timeout отменяет контекст через
	// 30 с, что закрыло бы все долгоживущие WS-соединения.
	r.Group(func(r chi.Router) {
		r.Use(chimw.Timeout(30 * time.Second))

		authH := handler.NewAuthHandler(d.Auth, d.Cfg)
		r.Route("/api/auth", func(r chi.Router) {
			r.Post("/register", authH.Register)
			r.Post("/login", authH.Login)
			r.Post("/refresh", authH.Refresh)

			r.Group(func(r chi.Router) {
				r.Use(mw.Auth(d.Tokens))
				r.Post("/logout", authH.Logout)
				r.Get("/me", authH.Me)
			})
		})

		mountDisciplines(r, d)
		mountNotificationsREST(r, d)
	})

	// WebSocket — без таймаута, соединение живёт пока клиент не отключится.
	mountNotificationsWS(r, d)

	return r
}

// mountDisciplines регистрирует /api/disciplines/* с RBAC-защитой:
//   - GET-эндпоинты: любой авторизованный + permission disciplines.view
//   - POST/PATCH/DELETE: соответствующие disciplines.create/update/delete
func mountDisciplines(r chi.Router, d Deps) {
	h := handler.NewDisciplineHandler(d.Disciplines)

	r.Route("/api/disciplines", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))

		// Просмотр справочника — доступно любому авторизованному,
		// без отдельного permission. Студент должен знать состав
		// дисциплин в системе, а в сидах disciplines.view выдан
		// только teacher/dean/admin (отдельная permission под
		// "управление справочником"). По бизнес-смыслу это
		// публичная информация внутри системы.
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.Get("/{id}/teachers", h.ListTeachers)
		r.Get("/{id}/students", h.ListStudents)

		// Создание — только admin/dean (через disciplines.create).
		r.With(mw.RequirePermission(d.RBAC, "disciplines.create")).
			Post("/", h.Create)

		// Обновление — admin/dean (через disciplines.update).
		r.With(mw.RequirePermission(d.RBAC, "disciplines.update")).
			Patch("/{id}", h.Update)

		// Удаление / restore — только admin (disciplines.delete).
		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "disciplines.delete"))
			r.Delete("/{id}", h.Delete)
			r.Post("/{id}/restore", h.Restore)
		})

		// Привязка преподавателей — admin/dean.
		// Берём disciplines.update — это редактирование состава
		// дисциплины. Отдельный permission заводить не стали:
		// действие "назначить препода на курс" в курсаче приходится
		// именно деканату, у которого уже есть update.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePermission(d.RBAC, "disciplines.update"))
			r.Post("/{id}/teachers", h.AttachTeacher)
			r.Delete("/{id}/teachers/{user_id}", h.DetachTeacher)
			r.Post("/{id}/students", h.AttachStudent)
			r.Delete("/{id}/students/{user_id}", h.DetachStudent)
		})
	})
}

// mountNotificationsREST регистрирует /api/notifications/* (под таймаутом).
func mountNotificationsREST(r chi.Router, d Deps) {
	notifH := handler.NewNotificationsHandler(d.Notify)
	r.Route("/api/notifications", func(r chi.Router) {
		r.Use(mw.Auth(d.Tokens))
		r.Get("/", notifH.List)
		r.Get("/unread-count", notifH.UnreadCount)
		r.Post("/{id}/read", notifH.MarkRead)
	})
}

// mountNotificationsWS регистрирует /ws/notifications без таймаута.
// Auth происходит внутри хендлера через query-параметр token.
func mountNotificationsWS(r chi.Router, d Deps) {
	wsH := handler.NewWSHandler(d.Tokens, d.Notify, d.NotifyHub, d.Cfg.AllowedOrigins)
	r.Get("/ws/notifications", wsH.ServeNotifications)
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
