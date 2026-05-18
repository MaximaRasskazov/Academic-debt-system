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

// List — GET /api/disciplines?limit=&offset=
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

// Get — GET /api/disciplines/:id
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

// Create — POST /api/disciplines
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

// Update — PATCH /api/disciplines/:id
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

// Delete — DELETE /api/disciplines/:id (soft-delete)
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

// Restore — POST /api/disciplines/:id/restore
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

// ListTeachers — GET /api/disciplines/:id/teachers
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

// AttachTeacher — POST /api/disciplines/:id/teachers
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

// DetachTeacher — DELETE /api/disciplines/:id/teachers/:user_id
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

// ListStudents — GET /api/disciplines/:id/students
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

// AttachStudent — POST /api/disciplines/:id/students
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

// DetachStudent — DELETE /api/disciplines/:id/students/:user_id
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
