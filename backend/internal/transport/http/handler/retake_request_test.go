package handler_test

// Unit-тесты handler-слоя /api/retake-requests/*.
//
// Не поднимают БД: используют stub-имплементацию RetakeRequestService.
// Auth-middleware намеренно НЕ применяется (тестируем только handler);
// 401 проверяется отсутствием UserID в context, остальные кейсы
// инжектят userID через mw.WithUserContext.
//
// Те же паттерны, что в retake_test.go: chi-роутер с теми же
// шаблонами, что в router_retake_requests.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retakerequest"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// retakeRequestServiceStub — программируемый stub.
// Каждый тест ставит ровно те fn, которые ему нужны.
type retakeRequestServiceStub struct {
	submitFn      func(ctx context.Context, actorID uuid.UUID, p retakerequest.Payload) (retakerequest.Request, error)
	listPendingFn func(ctx context.Context, limit, offset int32) ([]retakerequest.Request, error)
	listMyFn      func(ctx context.Context, teacherID uuid.UUID, limit, offset int32) ([]retakerequest.Request, error)
	approveFn     func(ctx context.Context, id, actorID uuid.UUID, decisionReason *string) (retakerequest.Request, error)
	rejectFn      func(ctx context.Context, id, actorID uuid.UUID, decisionReason string) (retakerequest.Request, error)
}

func (s *retakeRequestServiceStub) Submit(ctx context.Context, actorID uuid.UUID, p retakerequest.Payload) (retakerequest.Request, error) {
	return s.submitFn(ctx, actorID, p)
}
func (s *retakeRequestServiceStub) ListPending(ctx context.Context, limit, offset int32) ([]retakerequest.Request, error) {
	return s.listPendingFn(ctx, limit, offset)
}
func (s *retakeRequestServiceStub) ListMy(ctx context.Context, teacherID uuid.UUID, limit, offset int32) ([]retakerequest.Request, error) {
	return s.listMyFn(ctx, teacherID, limit, offset)
}
func (s *retakeRequestServiceStub) Approve(ctx context.Context, id, actorID uuid.UUID, decisionReason *string) (retakerequest.Request, error) {
	return s.approveFn(ctx, id, actorID, decisionReason)
}
func (s *retakeRequestServiceStub) Reject(ctx context.Context, id, actorID uuid.UUID, decisionReason string) (retakerequest.Request, error) {
	return s.rejectFn(ctx, id, actorID, decisionReason)
}

// retakeRequestFixture — мини-роутер с теми же шаблонами, что в router.
func retakeRequestFixture(h *handler.RetakeRequestHandler) *chi.Mux {
	r := chi.NewMux()
	r.Post("/api/retake-requests/", h.Submit)
	r.Get("/api/retake-requests/", h.ListPending)
	r.Get("/api/retake-requests/my", h.ListMy)
	r.Post("/api/retake-requests/{id}/approve", h.Approve)
	r.Post("/api/retake-requests/{id}/reject", h.Reject)
	return r
}

// ── Submit ────────────────────────────────────────────────────

func TestRetakeRequestHandler_Submit_OK(t *testing.T) {
	actor := uuid.New()
	stub := &retakeRequestServiceStub{
		submitFn: func(_ context.Context, gotActor uuid.UUID, p retakerequest.Payload) (retakerequest.Request, error) {
			require.Equal(t, actor, gotActor)
			require.Equal(t, "commission", p.Kind)
			require.Equal(t, int32(90), p.DurationMinutes)
			require.Len(t, p.TeacherIDs, 3)
			return retakerequest.Request{
				ID:          uuid.New(),
				RequestedBy: gotActor,
				Payload:     p,
				Status:      "pending",
				CreatedAt:   time.Now(),
			}, nil
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	teacher1, teacher2, teacher3 := uuid.New(), uuid.New(), uuid.New()
	body := map[string]any{
		"discipline_id":    uuid.New().String(),
		"kind":             "commission",
		"scheduled_at":     time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"duration_minutes": 90,
		"building":         "1",
		"room":             "204",
		"teacher_ids":      []string{teacher1.String(), teacher2.String(), teacher3.String()},
		"reason":           "много студентов на пересдаче",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/", bytes.NewReader(raw))
	req = req.WithContext(mw.WithUserContext(req.Context(), actor, uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "pending", resp["status"])
}

func TestRetakeRequestHandler_Submit_Unauthorized(t *testing.T) {
	h := handler.NewRetakeRequestHandler(&retakeRequestServiceStub{})
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/", strings.NewReader(`{}`))
	// userID НЕ кладём в контекст.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRetakeRequestHandler_Submit_InvalidInput(t *testing.T) {
	stub := &retakeRequestServiceStub{
		submitFn: func(context.Context, uuid.UUID, retakerequest.Payload) (retakerequest.Request, error) {
			return retakerequest.Request{}, retakerequest.ErrInvalidInput
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/", strings.NewReader(`{}`))
	req = req.WithContext(mw.WithUserContext(req.Context(), uuid.New(), uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRetakeRequestHandler_Submit_InvalidKind(t *testing.T) {
	stub := &retakeRequestServiceStub{
		submitFn: func(context.Context, uuid.UUID, retakerequest.Payload) (retakerequest.Request, error) {
			return retakerequest.Request{}, retakerequest.ErrInvalidKind
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/",
		strings.NewReader(`{"kind":"strange"}`))
	req = req.WithContext(mw.WithUserContext(req.Context(), uuid.New(), uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_kind")
}

// ── ListPending / ListMy ──────────────────────────────────────

func TestRetakeRequestHandler_ListPending_OK(t *testing.T) {
	stub := &retakeRequestServiceStub{
		listPendingFn: func(_ context.Context, limit, offset int32) ([]retakerequest.Request, error) {
			assert.Equal(t, int32(50), limit)
			assert.Equal(t, int32(0), offset)
			return []retakerequest.Request{{
				ID:        uuid.New(),
				Status:    "pending",
				CreatedAt: time.Now(),
			}}, nil
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodGet, "/api/retake-requests/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
}

func TestRetakeRequestHandler_ListMy_OK(t *testing.T) {
	actor := uuid.New()
	stub := &retakeRequestServiceStub{
		listMyFn: func(_ context.Context, teacherID uuid.UUID, _, _ int32) ([]retakerequest.Request, error) {
			require.Equal(t, actor, teacherID)
			return []retakerequest.Request{}, nil
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodGet, "/api/retake-requests/my", nil)
	req = req.WithContext(mw.WithUserContext(req.Context(), actor, uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRetakeRequestHandler_ListMy_Unauthorized(t *testing.T) {
	h := handler.NewRetakeRequestHandler(&retakeRequestServiceStub{})
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodGet, "/api/retake-requests/my", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── Approve ───────────────────────────────────────────────────

func TestRetakeRequestHandler_Approve_OK(t *testing.T) {
	id := uuid.New()
	actor := uuid.New()
	stub := &retakeRequestServiceStub{
		approveFn: func(_ context.Context, gotID, gotActor uuid.UUID, _ *string) (retakerequest.Request, error) {
			require.Equal(t, id, gotID)
			require.Equal(t, actor, gotActor)
			return retakerequest.Request{ID: gotID, Status: "approved", CreatedAt: time.Now()}, nil
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/"+id.String()+"/approve", strings.NewReader(`{}`))
	req = req.WithContext(mw.WithUserContext(req.Context(), actor, uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}

func TestRetakeRequestHandler_Approve_AlreadyProcessed(t *testing.T) {
	stub := &retakeRequestServiceStub{
		approveFn: func(context.Context, uuid.UUID, uuid.UUID, *string) (retakerequest.Request, error) {
			return retakerequest.Request{}, retakerequest.ErrNotPending
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/"+uuid.NewString()+"/approve", strings.NewReader(`{}`))
	req = req.WithContext(mw.WithUserContext(req.Context(), uuid.New(), uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestRetakeRequestHandler_Approve_NotFound(t *testing.T) {
	stub := &retakeRequestServiceStub{
		approveFn: func(context.Context, uuid.UUID, uuid.UUID, *string) (retakerequest.Request, error) {
			return retakerequest.Request{}, retakerequest.ErrNotFound
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/"+uuid.NewString()+"/approve", strings.NewReader(`{}`))
	req = req.WithContext(mw.WithUserContext(req.Context(), uuid.New(), uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRetakeRequestHandler_Approve_BadID(t *testing.T) {
	h := handler.NewRetakeRequestHandler(&retakeRequestServiceStub{})
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/not-a-uuid/approve", strings.NewReader(`{}`))
	req = req.WithContext(mw.WithUserContext(req.Context(), uuid.New(), uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Reject ────────────────────────────────────────────────────

func TestRetakeRequestHandler_Reject_OK(t *testing.T) {
	id := uuid.New()
	stub := &retakeRequestServiceStub{
		rejectFn: func(_ context.Context, gotID, _ uuid.UUID, reason string) (retakerequest.Request, error) {
			require.Equal(t, id, gotID)
			require.Equal(t, "не сегодня", reason)
			return retakerequest.Request{ID: gotID, Status: "rejected", CreatedAt: time.Now()}, nil
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	body := `{"decision_reason":"не сегодня"}`
	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/"+id.String()+"/reject", strings.NewReader(body))
	req = req.WithContext(mw.WithUserContext(req.Context(), uuid.New(), uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}

func TestRetakeRequestHandler_Reject_EmptyReason(t *testing.T) {
	// Сервис вернёт ErrInvalidInput для пустой причины — handler должен
	// замаппить в 400 invalid_input. (Бэк-валидация в retakerequest.Reject
	// уже проверяет TrimSpace(reason)=="".)
	stub := &retakeRequestServiceStub{
		rejectFn: func(_ context.Context, _, _ uuid.UUID, reason string) (retakerequest.Request, error) {
			require.Equal(t, "", reason)
			return retakerequest.Request{}, errors.Join(retakerequest.ErrInvalidInput, errors.New("причина отклонения обязательна"))
		},
	}
	h := handler.NewRetakeRequestHandler(stub)
	r := retakeRequestFixture(h)

	req := httptest.NewRequest(http.MethodPost, "/api/retake-requests/"+uuid.NewString()+"/reject", strings.NewReader(`{}`))
	req = req.WithContext(mw.WithUserContext(req.Context(), uuid.New(), uuid.New()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
