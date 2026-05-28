package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/teacherrequest"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// TeacherRequestHandler обслуживает /api/teacher-requests/*.
type TeacherRequestHandler struct {
	svc *teacherrequest.Service
}

func NewTeacherRequestHandler(svc *teacherrequest.Service) *TeacherRequestHandler {
	return &TeacherRequestHandler{svc: svc}
}

// ListPending — GET /api/teacher-requests. Деканат просматривает pending-заявки.
func (h *TeacherRequestHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	rows, err := h.svc.ListPending(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось загрузить заявки")
		return
	}
	writeJSON(w, http.StatusOK, dto.TeacherRequestsListResponse{
		Items: dto.FromTeacherRequests(rows), Limit: limit, Offset: offset,
	})
}

// Approve — POST /api/teacher-requests/:id/approve. Одобрение + выдача роли.
func (h *TeacherRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
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
		mapTeacherRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromTeacherRequest(teacherrequest.WithUser{TeacherRoleRequest: result}))
}

// Reject — POST /api/teacher-requests/:id/reject. Отклонение с причиной.
func (h *TeacherRequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
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
		mapTeacherRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromTeacherRequest(teacherrequest.WithUser{TeacherRoleRequest: result}))
}

func mapTeacherRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, teacherrequest.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "заявка не найдена")
	case errors.Is(err, teacherrequest.ErrAlreadyReviewed):
		writeError(w, http.StatusConflict, "already_reviewed", "заявка уже рассмотрена")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка сервиса")
	}
}
