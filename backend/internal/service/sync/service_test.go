package sync_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/emulator"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	syncsvc "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/sync"
)

// systemUserUUID — UUID служебного пользователя из сид-миграции 00010_seed_rbac.
var systemUserUUID = pgutil.PgUUID(uuid.MustParse("00000000-0000-0000-0000-000000000000"))

// setupSync поднимает sync.Service против httptest-эмулятора.
// Если TEST_DATABASE_URL не задан — тест пропускается, потому что
// Sync читает/пишет в sync_state.
func setupSync(t *testing.T, handler http.HandlerFunc) (*syncsvc.Service, *repo.Store, *httptest.Server) {
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

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	store := repo.NewStore(pool)
	client := emulator.New(srv.URL, "test-key")
	svc := syncsvc.New(store, client, systemUserUUID)
	return svc, store, srv
}

// setLastSynced ставит произвольную метку времени в sync_state.
// Возвращает выставленное значение для последующих assertion'ов.
func setLastSynced(t *testing.T, store *repo.Store, at time.Time) time.Time {
	t.Helper()
	ctx := context.Background()
	_, err := store.Pool().Exec(ctx,
		`UPDATE sync_state SET last_synced_at = $1 WHERE key = 'emulator'`, at)
	require.NoError(t, err)
	return at
}

// TestSync_RateLimited_DoesNotAdvanceLastSyncedAt: если эмулятор отвечает
// 429 на запрос /changes, last_synced_at не должен двигаться.
// Иначе при следующем тике мы попросили бы more recent since и потеряли
// бы change'ы которые упали по rate limit.
//
// Раньше тест ловил 429 на индивидуальном GET /debts/:id, который sync
// делал для каждой записи в applyChange. Сейчас sync парсит
// change.NewValue без дополнительных HTTP-запросов, поэтому 429 возможен
// только на самом /changes — это поведение мы здесь и проверяем.
func TestSync_RateLimited_DoesNotAdvanceLastSyncedAt(t *testing.T) {
	// Сокращаем backoff чтобы не ждать 5×35=175 сек на каждый запуск.
	// emulator.RetryAfter — экспортированная переменная пакета.
	prevRetry := emulator.RetryAfter
	prevMax := emulator.MaxAttempts
	emulator.RetryAfter = 10 * time.Millisecond
	emulator.MaxAttempts = 3
	t.Cleanup(func() {
		emulator.RetryAfter = prevRetry
		emulator.MaxAttempts = prevMax
	})

	handler := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/changes"):
			// Эмулятор перегружен — emulator/client.go после
			// MaxAttempts вернёт ErrRateLimited.
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[],"meta":{"page":1,"limit":100,"total":0,"total_pages":0}}`))
		}
	}
	svc, store, _ := setupSync(t, handler)
	ctx := context.Background()

	// Не-epoch значение, чтобы пропустить логику полного импорта.
	before := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	setLastSynced(t, store, before)

	err := svc.Sync(ctx)
	require.Error(t, err, "ожидали ошибку про rate limit")
	require.Contains(t, err.Error(), "rate limited")

	// Главное: last_synced_at НЕ сдвинулся.
	after, err := store.GetLastSyncedAt(ctx)
	require.NoError(t, err)
	require.WithinDuration(t, before, after, time.Second,
		"last_synced_at должен остаться прежним при rate limit")
}

// TestSync_NoChanges_AdvancesLastSyncedAt: контр-кейс — если
// change'ов нет и 429 нет, last_synced_at должен сдвинуться.
func TestSync_NoChanges_AdvancesLastSyncedAt(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/changes"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"next_since":"","has_more":false}}`))
		default:
			_, _ = w.Write([]byte(`{"data":[],"meta":{"page":1,"limit":100,"total":0,"total_pages":0}}`))
		}
	}
	svc, store, _ := setupSync(t, handler)
	ctx := context.Background()

	before := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	setLastSynced(t, store, before)

	require.NoError(t, svc.Sync(ctx))

	after, err := store.GetLastSyncedAt(ctx)
	require.NoError(t, err)
	require.True(t, after.After(before),
		"при отсутствии 429 last_synced_at должен сдвинуться вперёд")
}
