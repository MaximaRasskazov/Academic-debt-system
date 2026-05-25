package handler

import (
	"context"
	"net/http"
	"time"
)

// Syncer — интерфейс сервиса синхронизации с внешним эмулятором деканата.
// Определён здесь, чтобы handler не зависел от конкретной реализации:
// в тестах можно подменить заглушкой.
type Syncer interface {
	Sync(ctx context.Context) error
	LastSyncedAt(ctx context.Context) (time.Time, error)
}

// SyncHandler обслуживает /api/sync/*.
type SyncHandler struct {
	svc Syncer
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
	if err := h.svc.Sync(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "sync_failed", "синхронизация завершилась с ошибкой")
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
