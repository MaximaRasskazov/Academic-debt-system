// Package notify реализует систему уведомлений: запись в БД, real-time push
// через WebSocket и email через SMTP. Точка входа — Service.Notify, которая
// атомарно пишет в БД и best-effort рассылает в остальные каналы.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// Kind — slug события, идёт в поле kind таблицы notifications.
const (
	KindRetakeScheduled        = "retake_scheduled"
	KindRetakeUpdated          = "retake_updated"
	KindRetakeCancelled        = "retake_cancelled"
	KindRetakeGradeReceived    = "retake_grade_received"
	KindTeacherRequestApproved = "teacher_request_approved"
	KindTeacherRequestRejected = "teacher_request_rejected"

	KindRetakeChangeApproved = "retake_change_approved"
	KindRetakeChangeRejected = "retake_change_rejected"
)

// Event — входные данные для отправки уведомления.
type Event struct {
	UserID  uuid.UUID
	Kind    string
	Payload map[string]any
}

// Notification — доменная модель, независимая от sqlc-слоя.
type Notification struct {
	ID        uuid.UUID      `json:"id"`
	UserID    uuid.UUID      `json:"user_id"`
	Kind      string         `json:"kind"`
	Payload   map[string]any `json:"payload"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// EmailConfig — настройки SMTP для email-канала. Пустой Host отключает email.
type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Service — фасад для HTTP-слоя: рассылка + REST-операции над уведомлениями.
type Service struct {
	store    *repo.Store
	notifier *multiNotifier
	hub      *Hub
}

// NewService собирает Service с fan-out нотификатором.
// hub должен быть тем же экземпляром, что передаётся WS-хендлеру.
func NewService(store *repo.Store, hub *Hub, emailCfg EmailConfig) *Service {
	db := newDBNotifier(store)
	email := newEmailNotifier(emailCfg, store)
	return &Service{
		store:    store,
		notifier: newMultiNotifier(db, hub, email),
		hub:      hub,
	}
}

// Notify разворачивает уведомление по всем каналам.
func (s *Service) Notify(ctx context.Context, e Event) error {
	return s.notifier.notify(ctx, e)
}

// List возвращает историю уведомлений пользователя с пагинацией.
func (s *Service) List(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]Notification, error) {
	rows, err := s.store.ListNotificationsForUser(ctx, queries.ListNotificationsForUserParams{
		UserID: pgutil.PgUUID(userID),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("notify: list: %w", err)
	}
	return fromRows(rows), nil
}

// MarkRead помечает уведомление прочитанным. Идемпотентно.
// userID в SQL гарантирует, что нельзя прочитать чужое уведомление.
func (s *Service) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	return s.store.MarkNotificationRead(ctx, queries.MarkNotificationReadParams{
		ID:     pgutil.PgUUID(id),
		UserID: pgutil.PgUUID(userID),
	})
}

// CountUnread возвращает количество непрочитанных — для badge в шапке.
func (s *Service) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.store.CountUnreadForUser(ctx, pgutil.PgUUID(userID))
}

// ListUnread возвращает непрочитанные уведомления. Используется WS-хендлером
// для «burst» при подключении клиента после reconnect.
func (s *Service) ListUnread(ctx context.Context, userID uuid.UUID) ([]Notification, error) {
	rows, err := s.store.ListUnreadNotificationsForUser(ctx, pgutil.PgUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("notify: list unread: %w", err)
	}
	return fromRows(rows), nil
}

func fromRows(rows []queries.Notification) []Notification {
	out := make([]Notification, len(rows))
	for i, r := range rows {
		out[i] = fromRow(r)
	}
	return out
}

func fromRow(r queries.Notification) Notification {
	n := Notification{
		ID:        pgutil.UUID(r.ID),
		UserID:    pgutil.UUID(r.UserID),
		Kind:      r.Kind,
		CreatedAt: r.CreatedAt.Time,
	}
	if r.ReadAt.Valid {
		t := r.ReadAt.Time
		n.ReadAt = &t
	}
	if len(r.Payload) > 0 {
		_ = json.Unmarshal(r.Payload, &n.Payload)
	}
	return n
}
