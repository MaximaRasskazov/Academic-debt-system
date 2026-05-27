package notify_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
)

func testService(t *testing.T) (*repo.Store, *notify.Service) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан — интеграционный тест пропущен")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	store := repo.NewStore(pool)
	hub := notify.NewHub()
	// EmailConfig с пустым Host — email-канал отключён, тесты не ходят в SMTP.
	svc := notify.NewService(store, hub, notify.EmailConfig{})
	return store, svc
}

// seedUser создаёт временного пользователя и возвращает его uuid.UUID.
// CASCADE в схеме удалит связанные notifications при cleanup.
func seedUser(t *testing.T, store *repo.Store, prefix string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	email := strings.ToLower(prefix) + "+" + time.Now().Format("150405.000000") + "@test.local"
	u, err := store.CreateUser(ctx, queries.CreateUserParams{
		Email:        email,
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "T",
		LastName:     "N",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.Pool().Exec(ctx, "DELETE FROM users WHERE id = $1", u.ID)
	})
	return pgutil.UUID(u.ID)
}

func TestNotify_Notify_PersistsToDatabase(t *testing.T) {
	store, svc := testService(t)
	userID := seedUser(t, store, "notify-persist")
	ctx := context.Background()

	err := svc.Notify(ctx, notify.Event{
		UserID:  userID,
		Kind:    notify.KindRetakeScheduled,
		Payload: map[string]any{"discipline": "Математика", "scheduled_at": "2026-06-15 10:00"},
	})
	require.NoError(t, err)

	items, err := svc.List(ctx, userID, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, notify.KindRetakeScheduled, items[0].Kind)
	require.Equal(t, "Математика", items[0].Payload["discipline"])
	require.Nil(t, items[0].ReadAt, "новое уведомление не должно быть прочитанным")
}

func TestNotify_CountUnread_ReflectsNewNotifications(t *testing.T) {
	store, svc := testService(t)
	userID := seedUser(t, store, "notify-count")
	ctx := context.Background()

	count, err := svc.CountUnread(ctx, userID)
	require.NoError(t, err)
	require.EqualValues(t, 0, count)

	require.NoError(t, svc.Notify(ctx, notify.Event{UserID: userID, Kind: notify.KindRetakeScheduled}))
	require.NoError(t, svc.Notify(ctx, notify.Event{UserID: userID, Kind: notify.KindRetakeCancelled}))

	count, err = svc.CountUnread(ctx, userID)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)
}

func TestNotify_MarkRead_DecrementsUnreadCount(t *testing.T) {
	store, svc := testService(t)
	userID := seedUser(t, store, "notify-markread")
	ctx := context.Background()

	require.NoError(t, svc.Notify(ctx, notify.Event{UserID: userID, Kind: notify.KindRetakeUpdated}))

	items, err := svc.List(ctx, userID, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NoError(t, svc.MarkRead(ctx, items[0].ID, userID))

	count, err := svc.CountUnread(ctx, userID)
	require.NoError(t, err)
	require.EqualValues(t, 0, count)

	updated, err := svc.List(ctx, userID, 10, 0)
	require.NoError(t, err)
	require.NotNil(t, updated[0].ReadAt)
}

func TestNotify_MarkRead_IsIdempotent(t *testing.T) {
	store, svc := testService(t)
	userID := seedUser(t, store, "notify-idempotent")
	ctx := context.Background()

	require.NoError(t, svc.Notify(ctx, notify.Event{UserID: userID, Kind: notify.KindRetakeScheduled}))

	items, err := svc.List(ctx, userID, 10, 0)
	require.NoError(t, err)

	// Дважды — не должно ни паниковать, ни перезаписывать read_at.
	require.NoError(t, svc.MarkRead(ctx, items[0].ID, userID))
	require.NoError(t, svc.MarkRead(ctx, items[0].ID, userID))

	after, err := svc.List(ctx, userID, 10, 0)
	require.NoError(t, err)
	require.NotNil(t, after[0].ReadAt)
}

func TestNotify_MarkRead_CannotReadOthersNotification(t *testing.T) {
	// SQL-запрос содержит WHERE user_id=$2, поэтому exec не трогает чужие записи.
	store, svc := testService(t)
	ownerID := seedUser(t, store, "notify-owner")
	otherID := seedUser(t, store, "notify-other")
	ctx := context.Background()

	require.NoError(t, svc.Notify(ctx, notify.Event{UserID: ownerID, Kind: notify.KindRetakeScheduled}))

	items, err := svc.List(ctx, ownerID, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// Другой пользователь вызывает MarkRead — запрос тихо ничего не делает.
	require.NoError(t, svc.MarkRead(ctx, items[0].ID, otherID))

	count, err := svc.CountUnread(ctx, ownerID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count, "чужой MarkRead не должен затрагивать чужие уведомления")
}

func TestNotify_ListUnread_ReturnsOnlyUnread(t *testing.T) {
	store, svc := testService(t)
	userID := seedUser(t, store, "notify-unread")
	ctx := context.Background()

	require.NoError(t, svc.Notify(ctx, notify.Event{UserID: userID, Kind: notify.KindRetakeScheduled}))
	require.NoError(t, svc.Notify(ctx, notify.Event{UserID: userID, Kind: notify.KindRetakeUpdated}))

	all, err := svc.List(ctx, userID, 10, 0)
	require.NoError(t, err)
	require.Len(t, all, 2)

	require.NoError(t, svc.MarkRead(ctx, all[0].ID, userID))

	unread, err := svc.ListUnread(ctx, userID)
	require.NoError(t, err)
	require.Len(t, unread, 1, "должно остаться одно непрочитанное")
	require.Equal(t, all[1].ID, unread[0].ID)
}

// TestNotify_Notify_EmailAsync проверяет, что 100 параллельных вызовов
// Notify возвращаются быстро: email-канал асинхронный, SMTP не блокирует caller'а.
// Тест намеренно использует пустой EmailConfig — send() мгновенно вернёт nil,
// поэтому мы тестируем поведение очереди, а не SMTP.
func TestNotify_Notify_EmailAsync(t *testing.T) {
	store, svc := testService(t)
	defer svc.Close()

	userID := seedUser(t, store, "notify-async")
	ctx := context.Background()

	const n = 100
	done := make(chan error, n)
	start := time.Now()

	for range n {
		go func() {
			done <- svc.Notify(ctx, notify.Event{
				UserID: userID,
				Kind:   notify.KindRetakeScheduled,
			})
		}()
	}

	for range n {
		require.NoError(t, <-done)
	}

	elapsed := time.Since(start)
	require.Less(t, elapsed, 5*time.Second,
		"100 параллельных Notify заняли слишком долго: %v", elapsed)

	count, err := svc.CountUnread(ctx, userID)
	require.NoError(t, err)
	require.EqualValues(t, n, count, "все уведомления должны быть в БД")
}

func TestNotify_List_Pagination(t *testing.T) {
	store, svc := testService(t)
	userID := seedUser(t, store, "notify-page")
	ctx := context.Background()

	for i := range 5 {
		require.NoError(t, svc.Notify(ctx, notify.Event{
			UserID:  userID,
			Kind:    notify.KindRetakeScheduled,
			Payload: map[string]any{"i": i},
		}))
	}

	page1, err := svc.List(ctx, userID, 3, 0)
	require.NoError(t, err)
	require.Len(t, page1, 3)

	page2, err := svc.List(ctx, userID, 3, 3)
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// Нет пересечений между страницами.
	seen := make(map[uuid.UUID]bool)
	for _, n := range append(page1, page2...) {
		require.False(t, seen[n.ID], "дубликат ID на разных страницах")
		seen[n.ID] = true
	}
}
