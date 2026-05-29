package handler

import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/token"
)

// WSHandler обслуживает WebSocket-соединения для real-time уведомлений.
type WSHandler struct {
	tokens *token.Service
	svc    *notify.Service
	hub    *notify.Hub
}

func NewWSHandler(tokens *token.Service, svc *notify.Service, hub *notify.Hub, origins []string) *WSHandler {
	return &WSHandler{tokens: tokens, svc: svc, hub: hub}
}

// ServeNotifications — GET /ws/notifications?token=<access_token>
//
// Браузер не может устанавливать произвольные заголовки при WS-апгрейде,
// поэтому JWT приходит через query-параметр token.
func (h *WSHandler) ServeNotifications(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "token query param required")
		return
	}

	userID, _, err := h.tokens.Validate(r.Context(), tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
		return
	}

	// Origin-проверку делегируем JWT: токен в query-param и есть аутентификация.
	// OriginPatterns библиотеки ожидает host-паттерны (без схемы), а ALLOWED_ORIGINS
	// хранит полные URL — совместить их без потери гибкости сложнее, чем просто
	// доверять токену.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		// Accept сам пишет HTTP-ответ при ошибке апгрейда.
		return
	}
	defer func() { _ = conn.CloseNow() }()

	h.hub.Register(userID, conn)
	defer h.hub.Unregister(userID, conn)

	// Сразу шлём непрочитанные — клиент догоняет пропущенное после reconnect.
	if unread, err := h.svc.ListUnread(r.Context(), userID); err == nil {
		for _, n := range unread {
			if err := wsjson.Write(r.Context(), conn, n); err != nil {
				return
			}
		}
	}

	// Держим соединение открытым. Push идёт через Hub.broadcast при Notify.
	// Клиент → сервер: читаем только чтобы детектить disconnect.
	for {
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
	}
}
