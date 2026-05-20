package notify

import (
	"context"
	"log/slog"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

// Hub управляет активными WebSocket-соединениями.
// Один пользователь может быть подключён с нескольких вкладок —
// каждой выдаётся отдельный *websocket.Conn.
type Hub struct {
	mu    sync.RWMutex
	conns map[uuid.UUID][]*websocket.Conn
}

func NewHub() *Hub {
	return &Hub{conns: make(map[uuid.UUID][]*websocket.Conn)}
}

// Register добавляет соединение пользователя в хаб. Вызывается при апгрейде.
func (h *Hub) Register(userID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[userID] = append(h.conns[userID], conn)
}

// Unregister убирает соединение из хаба. Вызывается defer'ом после закрытия.
func (h *Hub) Unregister(userID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := h.conns[userID]
	for i, c := range list {
		if c == conn {
			h.conns[userID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(h.conns[userID]) == 0 {
		delete(h.conns, userID)
	}
}

// broadcast рассылает Notification всем активным соединениям пользователя.
// Ошибки записи в отдельный коннект логируются и не прерывают остальные.
func (h *Hub) broadcast(ctx context.Context, n Notification) error {
	h.mu.RLock()
	list := make([]*websocket.Conn, len(h.conns[n.UserID]))
	copy(list, h.conns[n.UserID])
	h.mu.RUnlock()

	for _, conn := range list {
		if err := wsjson.Write(ctx, conn, n); err != nil {
			slog.Warn("hub: write failed", "user_id", n.UserID, "err", err)
		}
	}
	return nil
}
