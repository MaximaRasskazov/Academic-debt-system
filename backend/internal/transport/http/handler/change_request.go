package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changerequest"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// ChangeRequestHandler обслуживает /api/retake-change-requests/*.
type ChangeRequestHandler struct {
	svc *changerequest.Service
}

func NewChangeRequestHandler(svc *changerequest.Service) *ChangeRequestHandler {
	return &ChangeRequestHandler{svc: svc}
}

// ListPending — GET /api/retake-change-requests. Деканат просматривает pending-заявки.
func (h *ChangeRequestHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	rows, err := h.svc.ListPending(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось загрузить заявки")
		return
	}
	writeJSON(w, http.StatusOK, dto.ChangeRequestsListResponse{
		Items: dto.FromChangeRequests(rows), Limit: limit, Offset: offset,
	})
}

// Approve — POST /api/retake-change-requests/:id/approve. Одобрение + применение изменений.
func (h *ChangeRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.svc.Approve(r.Context(), id, actorID)
	if err != nil {
		mapChangeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromChangeRequest(changerequest.WithUser{RetakeChangeRequest: result}))
}

// Reject — POST /api/retake-change-requests/:id/reject. Отклонение с причиной.
func (h *ChangeRequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var body dto.RejectRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	result, err := h.svc.Reject(r.Context(), id, actorID, body.Reason)
	if err != nil {
		mapChangeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromChangeRequest(changerequest.WithUser{RetakeChangeRequest: result}))
}

func mapChangeRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, changerequest.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "заявка не найдена")
	case errors.Is(err, changerequest.ErrAlreadyReviewed):
		writeError(w, http.StatusConflict, "already_reviewed", "заявка уже рассмотрена")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка сервиса")
	}
}
