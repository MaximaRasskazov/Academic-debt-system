package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retake"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// RetakeHandler собирает зависимости для /api/retakes/*.
type RetakeHandler struct {
	svc *retake.Service
}

func NewRetakeHandler(svc *retake.Service) *RetakeHandler {
	return &RetakeHandler{svc: svc}
}

// ListMy — GET /api/retakes/my. Permission retakes.view.own.
// Студент / преподаватель видят пересдачи где они участники.
func (h *RetakeHandler) ListMy(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	rows, err := h.svc.ListForUser(r.Context(), userID)
	if err != nil {
		mapRetakeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromRetakes(rows))
}

// ListAll — GET /api/retakes. Деканат видит всё с опциональным
// фильтром ?status=scheduled|in_progress|completed|cancelled.
func (h *RetakeHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)

	rows, err := h.svc.ListAll(r.Context(), status, limit, offset)
	if err != nil {
		mapRetakeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.RetakesListResponse{
		Items: dto.FromRetakes(rows), Limit: limit, Offset: offset,
	})
}

// Get — GET /api/retakes/:id.
func (h *RetakeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	row, err := h.svc.Get(r.Context(), id)
	if err != nil {
		mapRetakeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromRetake(row))
}

// Create — POST /api/retakes. Деканат создаёт пересдачу.
func (h *RetakeHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	var req dto.CreateRetakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	row, err := h.svc.Create(r.Context(), retake.CreateInput{
		DisciplineID:    req.DisciplineID,
		Kind:            req.Kind,
		Building:        req.Building,
		Room:            req.Room,
		ScheduledAt:     retake.NewTime(req.ScheduledAt),
		DurationMinutes: req.DurationMinutes,
		Notes:           req.Notes,
	}, userID)
	if err != nil {
		mapRetakeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.FromRetake(row))
}

// Update — PATCH /api/retakes/:id.
func (h *RetakeHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var req dto.UpdateRetakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	in := retake.UpdateScheduleInput{
		Building:        req.Building,
		Room:            req.Room,
		DurationMinutes: req.DurationMinutes,
		Notes:           req.Notes,
	}
	if req.ScheduledAt != nil {
		in.ScheduledAt = retake.NewTimePtr(req.ScheduledAt)
	}
	row, err := h.svc.UpdateSchedule(r.Context(), id, in, userID)
	if err != nil {
		mapRetakeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromRetake(row))
}

// Start — POST /api/retakes/:id/start. Деканат начинает пересдачу.
// Шедулер из BACK-07 будет делать это автоматически по scheduled_at.
func (h *RetakeHandler) Start(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Start(r.Context(), id, userID); err != nil {
		mapRetakeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Complete — POST /api/retakes/:id/complete. Ручное завершение.
func (h *RetakeHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Complete(r.Context(), id, userID); err != nil {
		mapRetakeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Cancel — POST /api/retakes/:id/cancel.
func (h *RetakeHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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
		mapRetakeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListParticipants — GET /api/retakes/:id/participants.
func (h *RetakeHandler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	rows, err := h.svc.ListParticipants(r.Context(), id)
	if err != nil {
		mapRetakeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromRetakeParticipants(rows))
}

// AddStudent — POST /api/retakes/:id/students.
func (h *RetakeHandler) AddStudent(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var req dto.AddRetakeStudentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	if err := h.svc.AddStudent(r.Context(), id, req.StudentID, req.DebtID, userID); err != nil {
		mapRetakeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddTeacher — POST /api/retakes/:id/teachers.
func (h *RetakeHandler) AddTeacher(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	id, ok := parseURLUUID(w, r, "id")
	if !ok {
		return
	}
	var req dto.AddRetakeTeacherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	if err := h.svc.AddTeacher(r.Context(), id, req.TeacherID, userID); err != nil {
		mapRetakeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveStudent — DELETE /api/retakes/:id/students/:user_id.
func (h *RetakeHandler) RemoveStudent(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
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
	if err := h.svc.RemoveStudent(r.Context(), id, studentID, userID); err != nil {
		mapRetakeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveTeacher — DELETE /api/retakes/:id/teachers/:user_id.
func (h *RetakeHandler) RemoveTeacher(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
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
	if err := h.svc.RemoveTeacher(r.Context(), id, teacherID, userID); err != nil {
		mapRetakeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GradeStudent — PATCH /api/retakes/:id/students/:user_id/grade.
// Выставление оценки атомарно закрывает связанный долг.
func (h *RetakeHandler) GradeStudent(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
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
	var req dto.GradeRetakeStudentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}
	if err := h.svc.GradeStudent(r.Context(), id, studentID, req.Grade, userID); err != nil {
		mapRetakeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Используем time для импорта - в случае если эта функция вырастет.
var _ = time.Time{}

// mapRetakeError маппит sentinel-ошибки в HTTP-коды.
func mapRetakeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, retake.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "пересдача не найдена")
	case errors.Is(err, retake.ErrParticipantNotFound):
		writeError(w, http.StatusNotFound, "participant_not_found", "участник не найден")
	case errors.Is(err, retake.ErrInvalidStatus):
		writeError(w, http.StatusConflict, "invalid_status", err.Error())
	case errors.Is(err, retake.ErrInvalidKind):
		writeError(w, http.StatusBadRequest, "invalid_kind", "kind должен быть regular или commission")
	case errors.Is(err, retake.ErrNotEnoughTeachers):
		writeError(w, http.StatusUnprocessableEntity, "not_enough_teachers", "для комиссии нужно минимум 3 преподавателя")
	case errors.Is(err, retake.ErrAlreadyParticipant):
		writeError(w, http.StatusConflict, "already_participant", "пользователь уже участник этой пересдачи")
	case errors.Is(err, retake.ErrStudentNeedsDebt):
		writeError(w, http.StatusBadRequest, "student_needs_debt", "студенту необходим debt_id")
	case errors.Is(err, retake.ErrParticipantNotStudent):
		writeError(w, http.StatusBadRequest, "not_student", "оценку можно ставить только студенту")
	case errors.Is(err, retake.ErrAlreadyHasGrade):
		writeError(w, http.StatusConflict, "already_has_grade", "оценка уже выставлена")
	case errors.Is(err, retake.ErrInvalidParticipant):
		writeError(w, http.StatusBadRequest, "invalid_participant", err.Error())
	case errors.Is(err, retake.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка сервиса")
	}
}
