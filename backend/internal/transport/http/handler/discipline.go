package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// DisciplineHandler собирает зависимости для /api/disciplines/*.
type DisciplineHandler struct {
	svc *discipline.Service
}

func NewDisciplineHandler(svc *discipline.Service) *DisciplineHandler {
	return &DisciplineHandler{svc: svc}
}

// List godoc
//
//	@Summary	Список дисциплин
//	@Tags		disciplines
//	@Produce	json
//	@Param		limit	query		int	false	"Лимит (default 50)"
//	@Param		offset	query		int	false	"Смещение (default 0)"
//	@Success	200		{object}	dto.DisciplinesListResponse
//	@Failure	401		{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines [get]
func (h *DisciplineHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	items, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		mapDisciplineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.DisciplinesListResponse{
		Items: dto.FromDisciplines(items), Total: total, Limit: limit, Offset: offset,
	})
}

// Get godoc
//
//	@Summary	Дисциплина по ID
//	@Tags		disciplines
//	@Produce	json
//	@Param		id	path		string	true	"UUID дисциплины"
//	@Success	200	{object}	dto.DisciplineResponse
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id} [get]
func (h *DisciplineHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	d, err := h.svc.Get(r.Context(), id)
	if err != nil {
		mapDisciplineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDiscipline(d))
}

// Create godoc
//
//	@Summary	Создать дисциплину
//	@Tags		disciplines
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.CreateDisciplineRequest	true	"Данные дисциплины"
//	@Success	201		{object}	dto.DisciplineResponse
//	@Failure	400		{object}	dto.ErrorResponse
//	@Failure	409		{object}	dto.ErrorResponse	"Code или Name уже заняты"
//	@Security	BearerAuth
//	@Router		/api/disciplines [post]
func (h *DisciplineHandler) Create(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	var req dto.CreateDisciplineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	d, err := h.svc.Create(r.Context(), discipline.CreateInput{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		ExternalID:  req.ExternalID,
		Source:      req.Source,
	}, actorID)
	if err != nil {
		mapDisciplineError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.FromDiscipline(d))
}

// Update godoc
//
//	@Summary	Обновить дисциплину
//	@Tags		disciplines
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"UUID дисциплины"
//	@Param		body	body		dto.UpdateDisciplineRequest	true	"Поля для обновления"
//	@Success	200		{object}	dto.DisciplineResponse
//	@Failure	404		{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id} [patch]
func (h *DisciplineHandler) Update(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var req dto.UpdateDisciplineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	d, err := h.svc.Update(r.Context(), id, discipline.UpdateInput{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
	}, actorID)
	if err != nil {
		mapDisciplineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromDiscipline(d))
}

// Delete godoc
//
//	@Summary	Удалить дисциплину (soft-delete)
//	@Tags		disciplines
//	@Param		id	path	string	true	"UUID дисциплины"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id} [delete]
func (h *DisciplineHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.SoftDelete(r.Context(), id, actorID); err != nil {
		mapDisciplineError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Restore godoc
//
//	@Summary	Восстановить дисциплину
//	@Tags		disciplines
//	@Param		id	path	string	true	"UUID дисциплины"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id}/restore [post]
func (h *DisciplineHandler) Restore(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Restore(r.Context(), id, actorID); err != nil {
		mapDisciplineError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTeachers godoc
//
//	@Summary	Преподаватели дисциплины
//	@Tags		disciplines
//	@Produce	json
//	@Param		id	path		string	true	"UUID дисциплины"
//	@Success	200	{array}		dto.UserResponse
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id}/teachers [get]
func (h *DisciplineHandler) ListTeachers(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	users, err := h.svc.ListTeachers(r.Context(), id)
	if err != nil {
		mapDisciplineError(w, err)
		return
	}
	out := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, dto.FromUser(u))
	}
	writeJSON(w, http.StatusOK, out)
}

// AttachTeacher godoc
//
//	@Summary	Назначить преподавателя на дисциплину
//	@Tags		disciplines
//	@Accept		json
//	@Param		id		path	string						true	"UUID дисциплины"
//	@Param		body	body	dto.AttachTeacherRequest	true	"ID преподавателя"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id}/teachers [post]
func (h *DisciplineHandler) AttachTeacher(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var req dto.AttachTeacherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	if err := h.svc.AttachTeacher(r.Context(), id, req.TeacherID, actorID); err != nil {
		mapDisciplineError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DetachTeacher godoc
//
//	@Summary	Снять преподавателя с дисциплины
//	@Tags		disciplines
//	@Param		id		path	string	true	"UUID дисциплины"
//	@Param		user_id	path	string	true	"UUID преподавателя"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id}/teachers/{user_id} [delete]
func (h *DisciplineHandler) DetachTeacher(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	teacherID, ok := parseURLUUID(w, r, "user_id")
	if !ok {
		return
	}
	if err := h.svc.DetachTeacher(r.Context(), id, teacherID, actorID); err != nil {
		mapDisciplineError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListStudents godoc
//
//	@Summary	Студенты дисциплины
//	@Tags		disciplines
//	@Produce	json
//	@Param		id	path		string	true	"UUID дисциплины"
//	@Success	200	{array}		dto.UserResponse
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id}/students [get]
func (h *DisciplineHandler) ListStudents(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	users, err := h.svc.ListStudents(r.Context(), id)
	if err != nil {
		mapDisciplineError(w, err)
		return
	}
	out := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, dto.FromUser(u))
	}
	writeJSON(w, http.StatusOK, out)
}

// AttachStudent godoc
//
//	@Summary	Зачислить студента на дисциплину
//	@Tags		disciplines
//	@Accept		json
//	@Param		id		path	string						true	"UUID дисциплины"
//	@Param		body	body	dto.AttachStudentRequest	true	"Студент и период обучения"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id}/students [post]
func (h *DisciplineHandler) AttachStudent(w http.ResponseWriter, r *http.Request) {
	actorID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var req dto.AttachStudentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	if err := h.svc.AttachStudent(r.Context(), id, req.StudentID, actorID, discipline.AttachStudentInput{
		AcademicYear: req.AcademicYear,
		Semester:     req.Semester,
		Source:       req.Source,
	}); err != nil {
		mapDisciplineError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DetachStudent godoc
//
//	@Summary	Отчислить студента с дисциплины
//	@Tags		disciplines
//	@Param		id		path	string	true	"UUID дисциплины"
//	@Param		user_id	path	string	true	"UUID студента"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/disciplines/{id}/students/{user_id} [delete]
func (h *DisciplineHandler) DetachStudent(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.DetachStudent(r.Context(), id, studentID, actorID); err != nil {
		mapDisciplineError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseURLUUID извлекает UUID из chi URL-параметра, отвечает 400 при невалидном.
func parseURLUUID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, key)
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", key+" не является UUID")
		return uuid.Nil, false
	}
	return id, true
}

// parseInt32 — для query-параметров пагинации.
func parseInt32(raw string, def int32) int32 {
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return def
	}
	return int32(v)
}

// mapDisciplineError маппит sentinel-ошибки сервиса в HTTP-коды.
func mapDisciplineError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, discipline.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "дисциплина не найдена")
	case errors.Is(err, discipline.ErrCodeTaken):
		writeError(w, http.StatusConflict, "code_taken", "code уже используется")
	case errors.Is(err, discipline.ErrNameTaken):
		writeError(w, http.StatusConflict, "name_taken", "name уже используется")
	case errors.Is(err, discipline.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "already_exists", "привязка уже существует")
	case errors.Is(err, discipline.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка сервиса")
	}
}
