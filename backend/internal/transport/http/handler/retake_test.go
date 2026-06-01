package handler_test

// Unit-тесты handler-слоя /api/retakes/*.
//
// Не поднимают БД: используют stub-имплементацию RetakeService.
// Auth-middleware намеренно НЕ применяется (тестируем только handler);
// 401 проверяется отсутствием UserID в context, остальные кейсы
// инжектят userID через mw.WithUserContext.
//
// Параметры пути chi-роутера передаются через chi.RouteContext —
// каждый тестовый запрос проходит через мини-роутер с тем же
// шаблоном, что и в router_retakes.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retake"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// retakeServiceStub реализует handler.RetakeService через программируемые
// функции — тест ставит ровно те, что ему нужны для конкретного кейса.
type retakeServiceStub struct {
	listForUserFn      func(ctx context.Context, userID uuid.UUID) ([]queries.Retake, error)
	listAllFn          func(ctx context.Context, status string, limit, offset int32) ([]queries.Retake, error)
	getFn              func(ctx context.Context, id uuid.UUID) (queries.Retake, error)
	createFn           func(ctx context.Context, in retake.CreateInput, actorID uuid.UUID) (queries.Retake, error)
	updateScheduleFn   func(ctx context.Context, id uuid.UUID, in retake.UpdateScheduleInput, actorID uuid.UUID) (queries.Retake, error)
	startFn            func(ctx context.Context, id, actorID uuid.UUID) error
	completeFn         func(ctx context.Context, id, actorID uuid.UUID) error
	cancelFn           func(ctx context.Context, id, actorID uuid.UUID) error
	listParticipantsFn func(ctx context.Context, id uuid.UUID) ([]queries.RetakeParticipant, error)
	addStudentFn       func(ctx context.Context, retakeID, studentID, debtID, actorID uuid.UUID) error
	addTeacherFn       func(ctx context.Context, retakeID, teacherID, actorID uuid.UUID) error
	removeStudentFn    func(ctx context.Context, retakeID, studentID, actorID uuid.UUID) error
	removeTeacherFn    func(ctx context.Context, retakeID, teacherID, actorID uuid.UUID) error
}

func (s *retakeServiceStub) ListForUser(ctx context.Context, userID uuid.UUID) ([]queries.Retake, error) {
	return s.listForUserFn(ctx, userID)
}
func (s *retakeServiceStub) ListAll(ctx context.Context, status string, limit, offset int32) ([]queries.Retake, error) {
	return s.listAllFn(ctx, status, limit, offset)
}
func (s *retakeServiceStub) Get(ctx context.Context, id uuid.UUID) (queries.Retake, error) {
	return s.getFn(ctx, id)
}
func (s *retakeServiceStub) Create(ctx context.Context, in retake.CreateInput, actorID uuid.UUID) (queries.Retake, error) {
	return s.createFn(ctx, in, actorID)
}
func (s *retakeServiceStub) UpdateSchedule(ctx context.Context, id uuid.UUID, in retake.UpdateScheduleInput, actorID uuid.UUID) (queries.Retake, error) {
	return s.updateScheduleFn(ctx, id, in, actorID)
}
func (s *retakeServiceStub) Start(ctx context.Context, id, actorID uuid.UUID) error {
	return s.startFn(ctx, id, actorID)
}
func (s *retakeServiceStub) Complete(ctx context.Context, id, actorID uuid.UUID) error {
	return s.completeFn(ctx, id, actorID)
}
func (s *retakeServiceStub) Cancel(ctx context.Context, id, actorID uuid.UUID) error {
	return s.cancelFn(ctx, id, actorID)
}
func (s *retakeServiceStub) ListParticipants(ctx context.Context, id uuid.UUID) ([]queries.RetakeParticipant, error) {
	return s.listParticipantsFn(ctx, id)
}
func (s *retakeServiceStub) AddStudent(ctx context.Context, retakeID, studentID, debtID, actorID uuid.UUID) error {
	return s.addStudentFn(ctx, retakeID, studentID, debtID, actorID)
}
func (s *retakeServiceStub) AddTeacher(ctx context.Context, retakeID, teacherID, actorID uuid.UUID) error {
	return s.addTeacherFn(ctx, retakeID, teacherID, actorID)
}
func (s *retakeServiceStub) RemoveStudent(ctx context.Context, retakeID, studentID, actorID uuid.UUID) error {
	return s.removeStudentFn(ctx, retakeID, studentID, actorID)
}
func (s *retakeServiceStub) RemoveTeacher(ctx context.Context, retakeID, teacherID, actorID uuid.UUID) error {
	return s.removeTeacherFn(ctx, retakeID, teacherID, actorID)
}

// retakeFixture — мини-роутер с одним методом, чтобы передать chi-параметры
// (например {id} в /api/retakes/:id) в handler.
func retakeFixture(h *handler.RetakeHandler) *chi.Mux {
	r := chi.NewMux()
	r.Get("/api/retakes/my", h.ListMy)
	r.Get("/api/retakes/", h.ListAll)
	r.Get("/api/retakes/{id}", h.Get)
	r.Get("/api/retakes/{id}/participants", h.ListParticipants)
	r.Post("/api/retakes/", h.Create)
	r.Patch("/api/retakes/{id}", h.Update)
	r.Post("/api/retakes/{id}/start", h.Start)
	r.Post("/api/retakes/{id}/complete", h.Complete)
	r.Post("/api/retakes/{id}/cancel", h.Cancel)
	r.Post("/api/retakes/{id}/students", h.AddStudent)
	r.Delete("/api/retakes/{id}/students/{user_id}", h.RemoveStudent)
	r.Post("/api/retakes/{id}/teachers", h.AddTeacher)
	r.Delete("/api/retakes/{id}/teachers/{user_id}", h.RemoveTeacher)
	return r
}

// withUser кладёт userID в context — имитирует прохождение через mw.Auth.
// Без него handler сразу вернёт 401.
func withUser(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := mw.WithUserContext(r.Context(), userID, uuid.New())
	return r.WithContext(ctx)
}

// pgUUID — pgtype.UUID из uuid.UUID для удобной сборки queries.Retake в тестах.
func pgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

// validRetake — заполненная queries.Retake-структура для positive-тестов.
func validRetake(id uuid.UUID) queries.Retake {
	return queries.Retake{
		ID:              pgUUID(id),
		DisciplineID:    pgUUID(uuid.New()),
		Kind:            "regular",
		Status:          "scheduled",
		MinTeachers:     1,
		Building:        "A",
		Room:            "101",
		ScheduledAt:     pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		DurationMinutes: 90,
		CreatedBy:       pgUUID(uuid.New()),
		CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// ── ListMy ──────────────────────────────────────────────────────────────────

func TestRetake_ListMy_401_NoAuth(t *testing.T) {
	h := handler.NewRetakeHandler(&retakeServiceStub{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/retakes/my", nil)
	retakeFixture(h).ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRetake_ListMy_200(t *testing.T) {
	uid := uuid.New()
	want := []queries.Retake{validRetake(uuid.New())}
	stub := &retakeServiceStub{
		listForUserFn: func(_ context.Context, gotUID uuid.UUID) ([]queries.Retake, error) {
			assert.Equal(t, uid, gotUID)
			return want, nil
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/retakes/my", nil), uid)
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
}

func TestRetake_ListMy_500_OnServiceError(t *testing.T) {
	stub := &retakeServiceStub{
		listForUserFn: func(_ context.Context, _ uuid.UUID) ([]queries.Retake, error) {
			return nil, errors.New("db connection refused")
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/retakes/my", nil), uuid.New())
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ── Get /:id ────────────────────────────────────────────────────────────────

func TestRetake_Get_400_InvalidUUID(t *testing.T) {
	h := handler.NewRetakeHandler(&retakeServiceStub{})
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/retakes/not-a-uuid", nil), uuid.New())
	retakeFixture(h).ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRetake_Get_404_NotFound(t *testing.T) {
	stub := &retakeServiceStub{
		getFn: func(_ context.Context, _ uuid.UUID) (queries.Retake, error) {
			return queries.Retake{}, retake.ErrNotFound
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/retakes/"+uuid.New().String(), nil), uuid.New())
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRetake_Get_200(t *testing.T) {
	id := uuid.New()
	stub := &retakeServiceStub{
		getFn: func(_ context.Context, gotID uuid.UUID) (queries.Retake, error) {
			assert.Equal(t, id, gotID)
			return validRetake(id), nil
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/retakes/"+id.String(), nil), uuid.New())
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
}

// ── Create ──────────────────────────────────────────────────────────────────

func TestRetake_Create_400_BadJSON(t *testing.T) {
	h := handler.NewRetakeHandler(&retakeServiceStub{})
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/retakes/",
		bytes.NewReader([]byte("not-json"))), uuid.New())
	retakeFixture(h).ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRetake_Create_400_InvalidKind(t *testing.T) {
	stub := &retakeServiceStub{
		createFn: func(_ context.Context, _ retake.CreateInput, _ uuid.UUID) (queries.Retake, error) {
			return queries.Retake{}, retake.ErrInvalidKind
		},
	}
	body, _ := json.Marshal(map[string]any{
		"discipline_id": uuid.New().String(),
		"kind":          "exam",
		"building":      "A", "room": "101",
		"scheduled_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"duration":     90,
	})
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/retakes/",
		bytes.NewReader(body)), uuid.New())
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRetake_Create_201(t *testing.T) {
	id := uuid.New()
	stub := &retakeServiceStub{
		createFn: func(_ context.Context, _ retake.CreateInput, _ uuid.UUID) (queries.Retake, error) {
			return validRetake(id), nil
		},
	}
	body, _ := json.Marshal(map[string]any{
		"discipline_id":    uuid.New().String(),
		"kind":             "regular",
		"building":         "A",
		"room":             "101",
		"scheduled_at":     time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"duration_minutes": 90,
	})
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/retakes/",
		bytes.NewReader(body)), uuid.New())
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, "body: %s", rr.Body.String())
}

// ── Start / Complete / Cancel ───────────────────────────────────────────────

func TestRetake_Start_422_InvalidStatus(t *testing.T) {
	// ErrInvalidStatus → 422 Unprocessable Entity (PR #40 от Андрея).
	// 409 Conflict зарезервирован под коллизии уникальности
	// (already_participant, email_taken и т.д.), а попытка стартовать
	// уже идущую пересдачу — это semantic error в бизнес-логике, не
	// конкурентный конфликт.
	stub := &retakeServiceStub{
		startFn: func(_ context.Context, _, _ uuid.UUID) error {
			return fmt.Errorf("уже идёт: %w", retake.ErrInvalidStatus)
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost,
		"/api/retakes/"+uuid.New().String()+"/start", nil), uuid.New())
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestRetake_Complete_204(t *testing.T) {
	stub := &retakeServiceStub{
		completeFn: func(_ context.Context, _, _ uuid.UUID) error { return nil },
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost,
		"/api/retakes/"+uuid.New().String()+"/complete", nil), uuid.New())
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestRetake_Cancel_404(t *testing.T) {
	stub := &retakeServiceStub{
		cancelFn: func(_ context.Context, _, _ uuid.UUID) error { return retake.ErrNotFound },
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost,
		"/api/retakes/"+uuid.New().String()+"/cancel", nil), uuid.New())
	retakeFixture(handler.NewRetakeHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code)
}
