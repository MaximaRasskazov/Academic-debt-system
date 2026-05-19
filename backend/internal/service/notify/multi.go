package notify

import (
	"context"
	"fmt"
	"log/slog"
)

// multiNotifier разворачивает Event по каналам:
//  1. БД (обязательно — без записи в историю остальное бессмысленно)
//  2. WebSocket (best-effort: пользователь может быть не подключён)
//  3. Email    (best-effort: SMTP может быть временно недоступен)
type multiNotifier struct {
	db    *dbNotifier
	hub   *Hub
	email *emailNotifier
}

func newMultiNotifier(db *dbNotifier, hub *Hub, email *emailNotifier) *multiNotifier {
	return &multiNotifier{db: db, hub: hub, email: email}
}

func (m *multiNotifier) notify(ctx context.Context, e Event) error {
	n, err := m.db.persist(ctx, e)
	if err != nil {
		return fmt.Errorf("notify: db: %w", err)
	}

	if err := m.hub.broadcast(ctx, n); err != nil {
		slog.Warn("notify: ws broadcast", "user_id", e.UserID, "kind", e.Kind, "err", err)
	}

	if err := m.email.send(ctx, e); err != nil {
		slog.Warn("notify: email", "user_id", e.UserID, "kind", e.Kind, "err", err)
	}

	return nil
}
