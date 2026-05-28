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
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	syncsvc "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/sync"
)

// systemUserUUID — UUID служебного пользователя из сид-миграции 00010_seed_rbac.
var systemUserUUID = uuid.MustParse("00000000-0000-0000-0000-000000000000")

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

// TestSync_RateLimited_DoesNotAdvanceLastSyncedAt: если все попытки
// получения долга падают с 429, last_synced_at должен остаться
// прежним — иначе при следующем тике мы потеряем эти change'ы.
func TestSync_RateLimited_DoesNotAdvanceLastSyncedAt(t *testing.T) {
	// Эмулятор отдаёт change-список с одной записью типа debt,
	// а на запрос самого debt'а — всегда 429.
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/changes"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [{"id":"c1","entity_type":"debt","entity_id":"d-1","action":"created","occurred_at":"2026-01-01T00:00:00Z","new_value":{}}],
				"meta": {"next_since":"","has_more":false}
			}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/debts/"):
			// Всегда 429 — backoff после 3 попыток вернёт ErrRateLimited.
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[],"meta":{"page":1,"limit":100,"total":0,"total_pages":0}}`))
		}
	}
	svc, store, _ := setupSync(t, handler)
	ctx := context.Background()

	// Фиксируем "стартовое" время и просим следующий цикл синка.
	// Берём не-epoch значение, чтобы пропустить полный импорт дисциплин.
	before := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	setLastSynced(t, store, before)

	err := svc.Sync(ctx)
	require.Error(t, err, "ожидали ошибку про rate limit")
	require.Contains(t, err.Error(), "rate limited")

	// Главная проверка: last_synced_at НЕ сдвинулся.
	after, err := store.GetLastSyncedAt(ctx)
	require.NoError(t, err)
	require.WithinDuration(t, before, after, time.Second,
		"last_synced_at должен остаться прежним, иначе потеряем change'ы")
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
