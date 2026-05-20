package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	teacherrequest "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/teacher_request"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// TeacherRequestHandler — хендлер заявок на роль преподавателя.
type TeacherRequestHandler struct {
	svc *teacherrequest.Service
}

func NewTeacherRequestHandler(svc *teacherrequest.Service) *TeacherRequestHandler {
	return &TeacherRequestHandler{svc: svc}
}

// Create — POST /api/teacher-requests.
// Любой авторизованный пользователь может подать заявку.
func (h *TeacherRequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "нет userID в контексте")
		return
	}

	var req dto.TeacherRequestCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON или повреждено")
		return
	}

	result, err := h.svc.Create(r.Context(), actorID, req.Reason)
	if err != nil {
		mapTeacherRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.FromTeacherRequest(result))
}

// ListPending — GET /api/teacher-requests.
// Деканат: список заявок ожидающих рассмотрения.
func (h *TeacherRequestHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	reqs, err := h.svc.ListPending(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось получить список заявок")
		return
	}
	writeJSON(w, http.StatusOK, dto.FromTeacherRequests(reqs))
}

// ListMy — GET /api/teacher-requests/my.
// История заявок текущего пользователя.
func (h *TeacherRequestHandler) ListMy(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "нет userID в контексте")
		return
	}
	limit, offset := parsePagination(r)
	reqs, err := h.svc.ListForUser(r.Context(), actorID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось получить список заявок")
		return
	}
	writeJSON(w, http.StatusOK, dto.FromTeacherRequests(reqs))
}

// Approve — POST /api/teacher-requests/:id/approve.
func (h *TeacherRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "нет userID в контексте")
		return
	}

	requestID, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "некорректный UUID заявки")
		return
	}

	var req dto.TeacherRequestApproveRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	result, err := h.svc.Approve(r.Context(), actorID, requestID, req.Reason)
	if err != nil {
		mapTeacherRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromTeacherRequest(result))
}

// Reject — POST /api/teacher-requests/:id/reject.
func (h *TeacherRequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "нет userID в контексте")
		return
	}

	requestID, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "некорректный UUID заявки")
		return
	}

	var req dto.TeacherRequestRejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON или повреждено")
		return
	}

	result, err := h.svc.Reject(r.Context(), actorID, requestID, req.Reason)
	if err != nil {
		mapTeacherRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromTeacherRequest(result))
}

func mapTeacherRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, teacherrequest.ErrAlreadyPending):
		writeError(w, http.StatusConflict, "already_pending", "у вас уже есть активная заявка")
	case errors.Is(err, teacherrequest.ErrRequestNotFound):
		writeError(w, http.StatusNotFound, "not_found", "заявка не найдена")
	case errors.Is(err, teacherrequest.ErrNotPending):
		writeError(w, http.StatusConflict, "not_pending", "заявка уже рассмотрена")
	case errors.Is(err, teacherrequest.ErrReasonRequired):
		writeError(w, http.StatusBadRequest, "reason_required", "причина отказа обязательна")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка сервиса")
	}
}

// parseUUIDParam достаёт UUID из chi URL-параметра.
func parseUUIDParam(r *http.Request, key string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, key))
}

// parsePagination читает limit/offset из query-параметров.
// Дефолты: limit=50, offset=0.
func parsePagination(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		var n int32
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		var n int32
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}