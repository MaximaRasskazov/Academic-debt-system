package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

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
//
// Вынесено отдельно от mountNotificationsREST потому что подключается
// в NewRouter вне group с chimw.Timeout(30s) — иначе через 30 с все
// долгоживущие WS-соединения принудительно закрывались бы.
func mountNotificationsWS(r chi.Router, d Deps) {
	wsH := handler.NewWSHandler(d.Tokens, d.Notify, d.NotifyHub, d.Cfg.AllowedOrigins)
	r.Get("/ws/notifications", wsH.ServeNotifications)
}
