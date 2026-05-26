package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/debt"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// DebtHandler собирает зависимости для /api/debts/*.
type DebtHandler struct {
	svc *debt.Service
}

func NewDebtHandler(svc *debt.Service) *DebtHandler {
	return &DebtHandler{svc: svc}
}

// ListMy godoc
//
//	@Summary	Мои долги (студент)
//	@Tags		debts
//	@Produce	json
//	@Success	200	{array}		dto.DebtResponse
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/debts/my [get]
func (h *DebtHandler) ListMy(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	rows, err := h.svc.ListForStudent(r.Context(), userID)
	if err != nil {
		mapDebtError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDebts(rows))
}

// ListByDiscipline godoc
//
//	@Summary	Долги по моим дисциплинам (преподаватель)
//	@Tags		debts
//	@Produce	json
//	@Param		limit	query		int	false	"Лимит"
//	@Param		offset	query		int	false	"Смещение"
//	@Success	200		{array}		dto.DebtResponse
//	@Failure	401		{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/debts/by-discipline [get]
func (h *DebtHandler) ListByDiscipline(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	rows, err := h.svc.ListForTeacher(r.Context(), userID, limit, offset)
	if err != nil {
		mapDebtError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDebts(rows))
}

// ListAll godoc
//
//	@Summary	Все долги (деканат)
//	@Tags		debts
//	@Produce	json
//	@Param		limit	query		int	false	"Лимит"
//	@Param		offset	query		int	false	"Смещение"
//	@Success	200		{object}	dto.DebtsListResponse
//	@Failure	401		{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/debts [get]
func (h *DebtHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	rows, total, err := h.svc.ListAll(r.Context(), limit, offset)
	if err != nil {
		mapDebtError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.DebtsListResponse{
		Items: dto.FromDebts(rows), Total: total, Limit: limit, Offset: offset,
	})
}

// Summary godoc
//
//	@Summary	Сводка долгов по дисциплинам
//	@Tags		debts
//	@Produce	json
//	@Success	200	{array}		dto.DebtSummaryRow
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/debts/summary [get]
func (h *DebtHandler) Summary(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.SummaryByDiscipline(r.Context())
	if err != nil {
		mapDebtError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDebtSummaries(rows))
}

// Get godoc
//
//	@Summary	Долг по ID
//	@Tags		debts
//	@Produce	json
//	@Param		id	path		string	true	"UUID долга"
//	@Success	200	{object}	dto.DebtResponse
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/debts/{id} [get]
func (h *DebtHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	row, err := h.svc.Get(r.Context(), id)
	if err != nil {
		mapDebtError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDebt(row))
}

// Create godoc
//
//	@Summary	Поставить долг (преподаватель)
//	@Tags		debts
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.CreateDebtRequest	true	"Данные долга"
//	@Success	201		{object}	dto.DebtResponse
//	@Failure	400		{object}	dto.ErrorResponse
//	@Failure	409		{object}	dto.ErrorResponse	"Открытый долг уже существует"
//	@Security	BearerAuth
//	@Router		/api/debts [post]
func (h *DebtHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	var req dto.CreateDebtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	row, err := h.svc.Create(r.Context(), debt.CreateInput{
		StudentID:    req.StudentID,
		DisciplineID: req.DisciplineID,
		ExternalID:   req.ExternalID,
		Source:       req.Source,
		Notes:        req.Notes,
	}, userID)
	if err != nil {
		mapDebtError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.FromDebt(row))
}

// Grade godoc
//
//	@Summary	Выставить оценку за долг
//	@Tags		debts
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"UUID долга"
//	@Param		body	body		dto.GradeDebtRequest	true	"Оценка (2-5)"
//	@Success	200		{object}	dto.DebtResponse
//	@Failure	400		{object}	dto.ErrorResponse
//	@Failure	409		{object}	dto.ErrorResponse	"Долг не в статусе open"
//	@Security	BearerAuth
//	@Router		/api/debts/{id}/grade [patch]
func (h *DebtHandler) Grade(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var req dto.GradeDebtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	row, err := h.svc.Grade(r.Context(), id, req.Grade, userID)
	if err != nil {
		mapDebtError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDebt(row))
}

// Cancel godoc
//
//	@Summary	Отменить долг (деканат)
//	@Tags		debts
//	@Param		id	path	string	true	"UUID долга"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/debts/{id}/cancel [patch]
func (h *DebtHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Cancel(r.Context(), id, userID); err != nil {
		mapDebtError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mapDebtError маппит sentinel-ошибки в HTTP-коды.
func mapDebtError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, debt.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "долг не найден")
	case errors.Is(err, debt.ErrTeacherNotAssigned):
		writeError(w, http.StatusForbidden, "teacher_not_assigned", "преподаватель не ведёт эту дисциплину")
	case errors.Is(err, debt.ErrStudentNotEnrolled):
		writeError(w, http.StatusUnprocessableEntity, "student_not_enrolled", "студент не учится на этой дисциплине")
	case errors.Is(err, debt.ErrDuplicateOpenDebt):
		writeError(w, http.StatusConflict, "duplicate_open_debt", "у студента уже есть открытый долг по этой дисциплине")
	case errors.Is(err, debt.ErrNotOpen):
		writeError(w, http.StatusConflict, "not_open", "долг не в статусе open")
	case errors.Is(err, debt.ErrInvalidGrade):
		writeError(w, http.StatusBadRequest, "invalid_grade", "оценка должна быть от 2 до 5")
	case errors.Is(err, debt.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка сервиса")
	}
}
