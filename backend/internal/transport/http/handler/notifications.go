package handler

import (
	"net/http"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// NotificationsHandler обслуживает REST-эндпоинты уведомлений.
type NotificationsHandler struct {
	svc *notify.Service
}

func NewNotificationsHandler(svc *notify.Service) *NotificationsHandler {
	return &NotificationsHandler{svc: svc}
}

// List godoc
//
//	@Summary	Список уведомлений
//	@Tags		notifications
//	@Produce	json
//	@Param		limit	query		int	false	"Лимит (default 20)"
//	@Param		offset	query		int	false	"Смещение"
//	@Success	200		{object}	dto.NotificationsListResponse
//	@Failure	401		{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/notifications [get]
func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	limit := parseInt32(r.URL.Query().Get("limit"), 20)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	items, err := h.svc.List(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось загрузить уведомления")
		return
	}
	writeJSON(w, http.StatusOK, dto.NotificationsListResponse{Items: dto.FromNotifications(items)})
}

// MarkRead godoc
//
//	@Summary	Отметить уведомление прочитанным
//	@Tags		notifications
//	@Param		id	path	string	true	"UUID уведомления"
//	@Success	204
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/notifications/{id}/read [post]
func (h *NotificationsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.MarkRead(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось отметить уведомление")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnreadCount godoc
//
//	@Summary	Счётчик непрочитанных уведомлений
//	@Tags		notifications
//	@Produce	json
//	@Success	200	{object}	dto.UnreadCountResponse
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/notifications/unread-count [get]
func (h *NotificationsHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	count, err := h.svc.CountUnread(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось получить счётчик")
		return
	}
	writeJSON(w, http.StatusOK, dto.UnreadCountResponse{Count: count})
}
