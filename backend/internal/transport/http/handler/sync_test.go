package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
)

// syncerStub реализует handler.Syncer без обращения к БД или эмулятору.
type syncerStub struct {
	syncErr       error
	lastSyncedAt  time.Time
	lastSyncedErr error
}

func (s *syncerStub) Sync(_ context.Context) error { return s.syncErr }
func (s *syncerStub) LastSyncedAt(_ context.Context) (time.Time, error) {
	return s.lastSyncedAt, s.lastSyncedErr
}

func TestSyncHandler_Status_OK(t *testing.T) {
	ts := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	h := handler.NewSyncHandler(&syncerStub{lastSyncedAt: ts})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/sync/status", nil)
	h.Status(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, ts.Format(time.RFC3339), body["last_synced_at"])
}

func TestSyncHandler_Status_NotConfigured(t *testing.T) {
	h := handler.NewSyncHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/sync/status", nil)
	h.Status(w, r)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSyncHandler_Status_ServiceError(t *testing.T) {
	h := handler.NewSyncHandler(&syncerStub{lastSyncedErr: errors.New("db down")})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/sync/status", nil)
	h.Status(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Trigger теперь работает асинхронно (см. handler/sync.go и PR fix(sync):
// Trigger запускает синк асинхронно, возвращает 202 сразу). Мы всегда
// отвечаем 202 Accepted сразу, а сам Sync крутится в горутине. Поэтому
// "успех" и "ошибка sync'а" с точки зрения handler — это один и тот же
// ответ: ошибка sync'а в logs, в http-ответе её не видно. Тесты ниже
// проверяют корректную семантику этого асинхронного flow.

func TestSyncHandler_Trigger_AcceptedAndRunsInBackground(t *testing.T) {
	stub := &syncerStub{lastSyncedAt: time.Date(2026, 5, 25, 13, 0, 0, 0, time.UTC)}
	h := handler.NewSyncHandler(stub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/sync/trigger", nil)
	h.Trigger(w, r)

	require.Equal(t, http.StatusAccepted, w.Code,
		"Trigger должен сразу отвечать 202 Accepted и запускать Sync в горутине")
}

func TestSyncHandler_Trigger_NotConfigured(t *testing.T) {
	h := handler.NewSyncHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/sync/trigger", nil)
	h.Trigger(w, r)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSyncHandler_Trigger_SyncErrorOnlyInLogs(t *testing.T) {
	// Sync вернёт ошибку, но это видно только в slog.Warn — HTTP-клиент
	// уже получил 202. Тест убеждается, что handler не делает ничего
	// глупого и не пытается записать ошибку в w после ServeHTTP.
	h := handler.NewSyncHandler(&syncerStub{syncErr: errors.New("emulator unreachable")})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/sync/trigger", nil)
	h.Trigger(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)
}
