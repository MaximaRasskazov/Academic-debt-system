package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retakerequest"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// RetakeRequestService — то, что нужно handler'у от сервиса заявок.
// Интерфейс объявлен здесь (не в пакете retakerequest), чтобы handler
// можно было тестировать через stub без поднятия БД. См. RetakeService
// в retake.go — тот же паттерн.
type RetakeRequestService interface {
	Submit(ctx context.Context, actorID uuid.UUID, p retakerequest.Payload) (retakerequest.Request, error)
	ListPending(ctx context.Context, limit, offset int32) ([]retakerequest.Request, error)
	ListMy(ctx context.Context, teacherID uuid.UUID, limit, offset int32) ([]retakerequest.Request, error)
	Approve(ctx context.Context, id, actorID uuid.UUID, decisionReason *string) (retakerequest.Request, error)
	Reject(ctx context.Context, id, actorID uuid.UUID, decisionReason string) (retakerequest.Request, error)
}

// RetakeRequestHandler обслуживает /api/retake-requests/*.
// Это заявки преподавателей на СОЗДАНИЕ пересдачи (отличается от
// /api/retake-change-requests, который про изменение существующей).
type RetakeRequestHandler struct {
	svc RetakeRequestService
}

func NewRetakeRequestHandler(svc RetakeRequestService) *RetakeRequestHandler {
	return &RetakeRequestHandler{svc: svc}
}

// Submit — POST /api/retake-requests.
// Преподаватель подаёт заявку.
func (h *RetakeRequestHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	var body dto.SubmitRetakeRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	result, err := h.svc.Submit(r.Context(), userID, retakerequest.Payload{
		DisciplineID:    body.DisciplineID,
		Kind:            body.Kind,
		ScheduledAt:     body.ScheduledAt,
		DurationMinutes: body.DurationMinutes,
		Building:        body.Building,
		Room:            body.Room,
		Notes:           body.Notes,
		Reason:          body.Reason,
		StudentDebtIDs:  body.StudentDebtIDs,
		TeacherIDs:      body.TeacherIDs,
	})
	if err != nil {
		mapRetakeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.FromRetakeRequest(result))
}

// ListPending — GET /api/retake-requests.
// Деканат видит все pending-заявки.
func (h *RetakeRequestHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	rows, err := h.svc.ListPending(r.Context(), limit, offset)
	if err != nil {
		mapRetakeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.RetakeRequestsListResponse{
		Items:  dto.FromRetakeRequests(rows),
		Limit:  limit,
		Offset: offset,
	})
}

// ListMy — GET /api/retake-requests/my.
// Преподаватель видит свои заявки.
func (h *RetakeRequestHandler) ListMy(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	rows, err := h.svc.ListMy(r.Context(), userID, limit, offset)
	if err != nil {
		mapRetakeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.RetakeRequestsListResponse{
		Items:  dto.FromRetakeRequests(rows),
		Limit:  limit,
		Offset: offset,
	})
}

// Approve — POST /api/retake-requests/{id}/approve.
// Деканат одобряет → создаётся пересдача + прикрепляются участники атомарно.
func (h *RetakeRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var body dto.DecisionBody
	// decision_reason опционален для approve — пустое тело валидно.
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	result, err := h.svc.Approve(r.Context(), id, userID, body.DecisionReason)
	if err != nil {
		mapRetakeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromRetakeRequest(result))
}

// Reject — POST /api/retake-requests/{id}/reject.
// decision_reason обязателен.
func (h *RetakeRequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var body dto.DecisionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	reason := ""
	if body.DecisionReason != nil {
		reason = *body.DecisionReason
	}
	result, err := h.svc.Reject(r.Context(), id, userID, reason)
	if err != nil {
		mapRetakeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromRetakeRequest(result))
}

// mapRetakeRequestError маппит sentinel-ошибки в HTTP-коды.
func mapRetakeRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, retakerequest.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "заявка не найдена")
	case errors.Is(err, retakerequest.ErrNotPending):
		writeError(w, http.StatusConflict, "already_processed", "заявка уже обработана")
	case errors.Is(err, retakerequest.ErrInvalidKind):
		writeError(w, http.StatusBadRequest, "invalid_kind", "kind должен быть regular или commission")
	case errors.Is(err, retakerequest.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, retakerequest.ErrScheduledInPast):
		writeError(w, http.StatusUnprocessableEntity, "scheduled_in_past",
			"время пересдачи уже прошло — отклоните заявку или попросите преподавателя подать новую")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка сервиса")
	}
}
