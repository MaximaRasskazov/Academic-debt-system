package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retake"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// RetakeService — узкий интерфейс ровно под методы, которые дёргает
// HTTP-handler. Сделан публичным чтобы тесты handler-слоя могли подсунуть
// stub без поднятия БД (см. retake_test.go). *retake.Service автоматически
// удовлетворяет интерфейс, поэтому wire-up в main.go и router.go не
// меняется.
type RetakeService interface {
	ListForUser(ctx context.Context, userID uuid.UUID) ([]queries.Retake, error)
	ListAll(ctx context.Context, status string, limit, offset int32) ([]queries.Retake, error)
	Get(ctx context.Context, id uuid.UUID) (queries.Retake, error)
	Create(ctx context.Context, in retake.CreateInput, actorID uuid.UUID) (queries.Retake, error)
	UpdateSchedule(ctx context.Context, id uuid.UUID, in retake.UpdateScheduleInput, actorID uuid.UUID) (queries.Retake, error)
	Start(ctx context.Context, id, actorID uuid.UUID) error
	Complete(ctx context.Context, id, actorID uuid.UUID) error
	Cancel(ctx context.Context, id, actorID uuid.UUID) error
	ListParticipants(ctx context.Context, retakeID uuid.UUID) ([]queries.RetakeParticipant, error)
	AddStudent(ctx context.Context, retakeID, studentID, debtID, actorID uuid.UUID) error
	AddTeacher(ctx context.Context, retakeID, teacherID, actorID uuid.UUID) error
	RemoveStudent(ctx context.Context, retakeID, studentID, actorID uuid.UUID) error
	RemoveTeacher(ctx context.Context, retakeID, teacherID, actorID uuid.UUID) error
	GradeStudent(ctx context.Context, retakeID, studentID uuid.UUID, grade int32, gradedBy uuid.UUID) error
}

// RetakeHandler собирает зависимости для /api/retakes/*.
type RetakeHandler struct {
	svc RetakeService
}

func NewRetakeHandler(svc RetakeService) *RetakeHandler {
	return &RetakeHandler{svc: svc}
}

// ListMy godoc
//
//	@Summary	Мои пересдачи (студент / преподаватель)
//	@Tags		retakes
//	@Produce	json
//	@Success	200	{array}		dto.RetakeResponse
//	@Failure	401	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/my [get]
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

// ListAll godoc
//
//	@Summary	Все пересдачи (деканат)
//	@Tags		retakes
//	@Produce	json
//	@Param		status	query		string	false	"Фильтр: scheduled|in_progress|completed|cancelled"
//	@Param		limit	query		int		false	"Лимит"
//	@Param		offset	query		int		false	"Смещение"
//	@Success	200		{object}	dto.RetakesListResponse
//	@Failure	401		{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes [get]
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

// Get godoc
//
//	@Summary	Пересдача по ID
//	@Tags		retakes
//	@Produce	json
//	@Param		id	path		string	true	"UUID пересдачи"
//	@Success	200	{object}	dto.RetakeResponse
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id} [get]
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

// Create godoc
//
//	@Summary	Создать пересдачу (деканат)
//	@Tags		retakes
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.CreateRetakeRequest	true	"Данные пересдачи"
//	@Success	201		{object}	dto.RetakeResponse
//	@Failure	400		{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes [post]
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

// Update godoc
//
//	@Summary	Обновить расписание пересдачи
//	@Tags		retakes
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"UUID пересдачи"
//	@Param		body	body		dto.UpdateRetakeRequest	true	"Поля для обновления"
//	@Success	200		{object}	dto.RetakeResponse
//	@Failure	404		{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id} [patch]
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

// Start godoc
//
//	@Summary	Начать пересдачу вручную
//	@Tags		retakes
//	@Param		id	path	string	true	"UUID пересдачи"
//	@Success	204
//	@Failure	409	{object}	dto.ErrorResponse	"Неверный статус"
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/start [post]
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

// Complete godoc
//
//	@Summary	Завершить пересдачу вручную
//	@Tags		retakes
//	@Param		id	path	string	true	"UUID пересдачи"
//	@Success	204
//	@Failure	409	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/complete [post]
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

// Cancel godoc
//
//	@Summary	Отменить пересдачу
//	@Tags		retakes
//	@Param		id	path	string	true	"UUID пересдачи"
//	@Success	204
//	@Failure	409	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/cancel [post]
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

// ListParticipants godoc
//
//	@Summary	Участники пересдачи
//	@Tags		retakes
//	@Produce	json
//	@Param		id	path		string	true	"UUID пересдачи"
//	@Success	200	{array}		dto.RetakeParticipantResponse
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/participants [get]
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

// AddStudent godoc
//
//	@Summary	Добавить студента на пересдачу
//	@Tags		retakes
//	@Accept		json
//	@Param		id		path	string						true	"UUID пересдачи"
//	@Param		body	body	dto.AddRetakeStudentRequest	true	"StudentID + DebtID"
//	@Success	204
//	@Failure	400	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/students [post]
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

// AddTeacher godoc
//
//	@Summary	Добавить преподавателя на пересдачу
//	@Tags		retakes
//	@Accept		json
//	@Param		id		path	string						true	"UUID пересдачи"
//	@Param		body	body	dto.AddRetakeTeacherRequest	true	"TeacherID"
//	@Success	204
//	@Failure	400	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/teachers [post]
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

// RemoveStudent godoc
//
//	@Summary	Убрать студента с пересдачи
//	@Tags		retakes
//	@Param		id		path	string	true	"UUID пересдачи"
//	@Param		user_id	path	string	true	"UUID студента"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/students/{user_id} [delete]
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

// RemoveTeacher godoc
//
//	@Summary	Убрать преподавателя с пересдачи
//	@Tags		retakes
//	@Param		id		path	string	true	"UUID пересдачи"
//	@Param		user_id	path	string	true	"UUID преподавателя"
//	@Success	204
//	@Failure	404	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/teachers/{user_id} [delete]
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

// GradeStudent godoc
//
//	@Summary	Выставить оценку студенту на пересдаче
//	@Description	Атомарно закрывает связанный долг студента.
//	@Tags		retakes
//	@Accept		json
//	@Param		id		path	string							true	"UUID пересдачи"
//	@Param		user_id	path	string							true	"UUID студента"
//	@Param		body	body	dto.GradeRetakeStudentRequest	true	"Оценка"
//	@Success	204
//	@Failure	400	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/retakes/{id}/students/{user_id}/grade [patch]
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
