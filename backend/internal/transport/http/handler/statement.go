package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/statement"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// StatementService — что нужно хэндлеру от сервиса ведомости.
// Объявлен здесь (не в statement), чтобы в тестах подсунуть stub
// без поднятия БД — тот же паттерн, что у DirectoryHandler/RetakeHandler.
type StatementService interface {
	GetSheet(ctx context.Context, retakeID uuid.UUID) (queries.StatementSheet, error)
	SaveDraftGrade(ctx context.Context, retakeID, studentID uuid.UUID, grade int32, actorID uuid.UUID) error
	CloseSheet(ctx context.Context, retakeID, actorID uuid.UUID) error
	ReopenSheet(ctx context.Context, retakeID, actorID uuid.UUID) error
}

// StatementHandler — /api/retakes/{id}/sheet/*.
type StatementHandler struct {
	svc StatementService
}

func NewStatementHandler(svc StatementService) *StatementHandler {
	return &StatementHandler{svc: svc}
}

// GetSheet godoc
//
//	@Summary	Статус ведомости пересдачи
//	@Tags		statements
//	@Produce	json
//	@Param		id	path		string	true	"UUID пересдачи"
//	@Success	200	{object}	dto.StatementSheetResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/sheet [get]
func (h *StatementHandler) GetSheet(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	sheet, err := h.svc.GetSheet(r.Context(), id)
	if err != nil {
		mapStatementError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromStatementSheet(sheet))
}

// SaveDraftGrade godoc
//
//	@Summary	Сохранить черновик оценки студенту (долг не закрывается)
//	@Tags		statements
//	@Accept		json
//	@Param		id		path	string						true	"UUID пересдачи"
//	@Param		user_id	path	string						true	"UUID студента"
//	@Param		body	body	dto.SaveDraftGradeRequest	true	"Оценка 2..5"
//	@Success	204
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/sheet/grades/{user_id} [patch]
func (h *StatementHandler) SaveDraftGrade(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	studentID, ok := parseURLUUID(w, r, "user_id")
	if !ok {
		return
	}
	var req dto.SaveDraftGradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	if err := h.svc.SaveDraftGrade(r.Context(), id, studentID, req.Grade, actorID); err != nil {
		mapStatementError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CloseSheet godoc
//
//	@Summary	Закрыть ведомость: зафиксировать оценки, закрыть долги
//	@Tags		statements
//	@Param		id	path	string	true	"UUID пересдачи"
//	@Success	204
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/sheet/close [post]
func (h *StatementHandler) CloseSheet(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.CloseSheet(r.Context(), id, actorID); err != nil {
		mapStatementError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReopenSheet godoc
//
//	@Summary	Открыть закрытую ведомость (только декан): откатить долги в open
//	@Tags		statements
//	@Param		id	path	string	true	"UUID пересдачи"
//	@Success	204
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/sheet/reopen [post]
func (h *StatementHandler) ReopenSheet(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.ReopenSheet(r.Context(), id, actorID); err != nil {
		mapStatementError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mapStatementError маппит sentinel-ошибки statement-сервиса в HTTP-коды.
func mapStatementError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, statement.ErrSheetClosed):
		writeError(w, http.StatusConflict, "sheet_closed", "ведомость закрыта")
	case errors.Is(err, statement.ErrSheetAlreadyOpen):
		writeError(w, http.StatusConflict, "sheet_already_open", "ведомость не закрыта")
	case errors.Is(err, statement.ErrRetakeNotGradeable):
		writeError(w, http.StatusUnprocessableEntity, "retake_not_gradeable", err.Error())
	case errors.Is(err, statement.ErrInvalidGrade):
		writeError(w, http.StatusBadRequest, "invalid_grade", "оценка должна быть 2..5")
	case errors.Is(err, statement.ErrNotStudent):
		writeError(w, http.StatusBadRequest, "not_student", "оценивать можно только студента")
	case errors.Is(err, statement.ErrStudentNeedsDebt):
		writeError(w, http.StatusBadRequest, "student_needs_debt", "у студента нет связанного долга")
	case errors.Is(err, statement.ErrParticipantNotFound):
		writeError(w, http.StatusNotFound, "participant_not_found", "участник не найден")
	case errors.Is(err, statement.ErrSheetNotFound), errors.Is(err, statement.ErrRetakeNotFound):
		writeError(w, http.StatusNotFound, "not_found", "пересдача или ведомость не найдена")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка")
	}
}
