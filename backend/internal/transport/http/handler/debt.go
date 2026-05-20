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

// ListMy — GET /api/debts/my. Студент видит свои долги.
// Здесь не нужен query-параметр student_id: берём userID из контекста.
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

// ListByDiscipline — GET /api/debts/by-discipline. Преподаватель
// видит долги по всем своим дисциплинам с пагинацией.
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

// ListAll — GET /api/debts. Деканат видит всё.
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

// Summary — GET /api/debts/summary. Сводная таблица деканата.
func (h *DebtHandler) Summary(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.SummaryByDiscipline(r.Context())
	if err != nil {
		mapDebtError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDebtSummaries(rows))
}

// Get — GET /api/debts/:id.
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

// Create — POST /api/debts. Преподаватель ставит долг.
// issuedBy берётся из контекста (текущий пользователь).
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

// Grade — PATCH /api/debts/:id/grade. Преподаватель выставляет оценку.
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

// Cancel — PATCH /api/debts/:id/cancel. Деканат отменяет долг.
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
