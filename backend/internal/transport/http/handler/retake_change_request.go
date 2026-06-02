package handler

import (
	"encoding/json"
	"errors"
	"io"
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

// Submit — POST /api/retake-change-requests.
// Преподаватель-участник подаёт заявку на изменение расписания пересдачи.
func (h *ChangeRequestHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	var req dto.SubmitChangeRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	result, err := h.svc.Submit(r.Context(), req.RetakeID, userID, changerequest.SubmitInput{
		Changes: req.Changes,
		Reason:  req.Reason,
	})
	if err != nil {
		mapChangeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.FromChangeRequest(result))
}

// ListPending — GET /api/retake-change-requests.
// Деканат видит все pending-заявки с пагинацией.
func (h *ChangeRequestHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	rows, err := h.svc.ListPending(r.Context(), limit, offset)
	if err != nil {
		mapChangeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.ChangeRequestsListResponse{
		Items:  dto.FromChangeRequests(rows),
		Limit:  limit,
		Offset: offset,
	})
}

// Approve — POST /api/retake-change-requests/:id/approve.
// Деканат одобряет заявку — изменения атомарно применяются к пересдаче.
func (h *ChangeRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.svc.Approve(r.Context(), id, userID, body.DecisionReason, body.SelectedSlot)
	if err != nil {
		mapChangeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromChangeRequest(result))
}

// Reject — POST /api/retake-change-requests/:id/reject.
// Деканат отклоняет заявку. decision_reason обязателен (требование ТЗ).
func (h *ChangeRequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
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
		mapChangeRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromChangeRequest(result))
}

// mapChangeRequestError маппит sentinel-ошибки в HTTP-коды.
func mapChangeRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, changerequest.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "заявка не найдена")
	case errors.Is(err, changerequest.ErrNotPending):
		writeError(w, http.StatusConflict, "already_processed", "заявка уже обработана")
	case errors.Is(err, changerequest.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "преподаватель не является участником пересдачи")
	case errors.Is(err, changerequest.ErrRetakeNotActive):
		writeError(w, http.StatusUnprocessableEntity, "retake_not_active", "пересдача завершена или отменена")
	case errors.Is(err, changerequest.ErrScheduledInPast):
		writeError(w, http.StatusUnprocessableEntity, "scheduled_in_past",
			"новое время пересдачи уже прошло — отклоните заявку или попросите преподавателя подать новую")
	case errors.Is(err, changerequest.ErrSlotRequired):
		writeError(w, http.StatusUnprocessableEntity, "slot_required",
			"преподаватель предложил несколько дат — откройте «Подробнее» и выберите одну")
	case errors.Is(err, changerequest.ErrSlotNotProposed):
		writeError(w, http.StatusBadRequest, "slot_not_proposed",
			"выбранная дата не входит в предложенные преподавателем")
	case errors.Is(err, changerequest.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка сервиса")
	}
}
