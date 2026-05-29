package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// Syncer — интерфейс сервиса синхронизации с внешним эмулятором деканата.
type Syncer interface {
	Sync(ctx context.Context) error
	LastSyncedAt(ctx context.Context) (time.Time, error)
}

// SyncHandler обслуживает /api/sync/*.
type SyncHandler struct {
	svc     Syncer
	running atomic.Bool // защита от параллельных запусков
}

// NewSyncHandler создаёт хендлер. svc может быть nil — тогда оба
// эндпоинта вернут 503, если EMULATOR_URL не задан в конфиге.
func NewSyncHandler(svc Syncer) *SyncHandler {
	return &SyncHandler{svc: svc}
}

// Status godoc
//
//	@Summary	Статус последней синхронизации
//	@Tags		sync
//	@Produce	json
//	@Success	200	{object}	map[string]string	"last_synced_at в формате RFC3339"
//	@Failure	401	{object}	dto.ErrorResponse
//	@Failure	503	{object}	dto.ErrorResponse	"синхронизация не настроена (EMULATOR_URL не задан)"
//	@Security	BearerAuth
//	@Router		/api/sync/status [get]
func (h *SyncHandler) Status(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		writeError(w, http.StatusServiceUnavailable, "sync_not_configured", "синхронизация с эмулятором не настроена")
		return
	}
	t, err := h.svc.LastSyncedAt(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось получить статус синхронизации")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"last_synced_at": t.Format(time.RFC3339),
	})
}

// Trigger godoc
//
//	@Summary	Запустить синхронизацию вручную
//	@Tags		sync
//	@Produce	json
//	@Success	200	{object}	map[string]string	"last_synced_at после завершения прогона"
//	@Failure	401	{object}	dto.ErrorResponse
//	@Failure	403	{object}	dto.ErrorResponse
//	@Failure	500	{object}	dto.ErrorResponse
//	@Failure	503	{object}	dto.ErrorResponse	"синхронизация не настроена"
//	@Security	BearerAuth
//	@Router		/api/sync/trigger [post]
func (h *SyncHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		writeError(w, http.StatusServiceUnavailable, "sync_not_configured", "синхронизация с эмулятором не настроена")
		return
	}
	if !h.running.CompareAndSwap(false, true) {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "already_running"})
		return
	}
	go func() {
		defer h.running.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := h.svc.Sync(ctx); err != nil {
			slog.Warn("sync: ручной запуск завершился с ошибкой", "err", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}
