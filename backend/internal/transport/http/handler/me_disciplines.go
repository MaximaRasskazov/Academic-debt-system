package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// UserDisciplinesHandler обслуживает endpoint'ы вида
// "дисциплины пользователя". Регистрируется в router и закрывает DoD
// BACK-01: фронт должен уметь спросить "на каких дисциплинах я учусь"
// (для студента) или "какие я веду" (для преподавателя).
type UserDisciplinesHandler struct {
	svc *discipline.Service
}

func NewUserDisciplinesHandler(svc *discipline.Service) *UserDisciplinesHandler {
	return &UserDisciplinesHandler{svc: svc}
}

// MyAsStudent godoc
//
//	@Summary	Мои дисциплины (как студент)
//	@Tags		disciplines
//	@Produce	json
//	@Success	200	{array}		dto.DisciplineResponse
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/me/disciplines/student [get]
func (h *UserDisciplinesHandler) MyAsStudent(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	h.listAsStudent(w, r, userID)
}

// MyAsTeacher godoc
//
//	@Summary	Мои дисциплины (как преподаватель)
//	@Tags		disciplines
//	@Produce	json
//	@Success	200	{array}		dto.DisciplineResponse
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/me/disciplines/teacher [get]
func (h *UserDisciplinesHandler) MyAsTeacher(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	h.listAsTeacher(w, r, userID)
}

// UserAsStudent godoc
//
//	@Summary	Дисциплины пользователя (как студент)
//	@Tags		disciplines
//	@Produce	json
//	@Param		id	path		string	true	"UUID пользователя"
//	@Success	200	{array}		dto.DisciplineResponse
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/users/{id}/disciplines/student [get]
func (h *UserDisciplinesHandler) UserAsStudent(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	h.listAsStudent(w, r, id)
}

// UserAsTeacher godoc
//
//	@Summary	Дисциплины пользователя (как преподаватель)
//	@Tags		disciplines
//	@Produce	json
//	@Param		id	path		string	true	"UUID пользователя"
//	@Success	200	{array}		dto.DisciplineResponse
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/users/{id}/disciplines/teacher [get]
func (h *UserDisciplinesHandler) UserAsTeacher(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	h.listAsTeacher(w, r, id)
}

func (h *UserDisciplinesHandler) listAsStudent(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	rows, err := h.svc.ListDisciplinesForStudent(r.Context(), userID)
	if err != nil {
		mapDisciplineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDisciplines(rows))
}

func (h *UserDisciplinesHandler) listAsTeacher(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	rows, err := h.svc.ListDisciplinesForTeacher(r.Context(), userID)
	if err != nil {
		mapDisciplineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDisciplines(rows))
}
