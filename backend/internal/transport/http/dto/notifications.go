package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
)

// NotificationResponse — публичное представление уведомления.
type NotificationResponse struct {
	ID        uuid.UUID      `json:"id"`
	Kind      string         `json:"kind"`
	Payload   map[string]any `json:"payload"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// NotificationsListResponse — ответ GET /api/notifications.
type NotificationsListResponse struct {
	Items []NotificationResponse `json:"items"`
}

// UnreadCountResponse — ответ GET /api/notifications/unread-count.
type UnreadCountResponse struct {
	Count int64 `json:"count"`
}

func FromNotification(n notify.Notification) NotificationResponse {
	return NotificationResponse{
		ID:        n.ID,
		Kind:      n.Kind,
		Payload:   n.Payload,
		ReadAt:    n.ReadAt,
		CreatedAt: n.CreatedAt,
	}
}

func FromNotifications(items []notify.Notification) []NotificationResponse {
	out := make([]NotificationResponse, len(items))
	for i, n := range items {
		out[i] = FromNotification(n)
	}
	return out
}
