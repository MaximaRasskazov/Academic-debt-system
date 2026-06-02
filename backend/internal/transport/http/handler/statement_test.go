package handler_test

// Unit-тесты StatementHandler — без БД, через statementServiceStub.
// Проверяют HTTP-семантику: коды, парсинг параметров, маппинг
// sentinel-ошибок. Бизнес-логика покрыта в internal/service/statement.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/statement"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
)

type statementServiceStub struct {
	getSheetFn   func(ctx context.Context, retakeID uuid.UUID) (queries.StatementSheet, error)
	saveDraftFn  func(ctx context.Context, retakeID, studentID uuid.UUID, grade int32, actorID uuid.UUID) error
	closeSheetFn func(ctx context.Context, retakeID, actorID uuid.UUID) error
	reopenFn     func(ctx context.Context, retakeID, actorID uuid.UUID) error
}

func (s *statementServiceStub) GetSheet(ctx context.Context, retakeID uuid.UUID) (queries.StatementSheet, error) {
	return s.getSheetFn(ctx, retakeID)
}
func (s *statementServiceStub) SaveDraftGrade(ctx context.Context, retakeID, studentID uuid.UUID, grade int32, actorID uuid.UUID) error {
	return s.saveDraftFn(ctx, retakeID, studentID, grade, actorID)
}
func (s *statementServiceStub) CloseSheet(ctx context.Context, retakeID, actorID uuid.UUID) error {
	return s.closeSheetFn(ctx, retakeID, actorID)
}
func (s *statementServiceStub) ReopenSheet(ctx context.Context, retakeID, actorID uuid.UUID) error {
	return s.reopenFn(ctx, retakeID, actorID)
}

func statementFixture(h *handler.StatementHandler) *chi.Mux {
	r := chi.NewMux()
	r.Get("/api/retakes/{id}/sheet", h.GetSheet)
	r.Patch("/api/retakes/{id}/sheet/grades/{user_id}", h.SaveDraftGrade)
	r.Post("/api/retakes/{id}/sheet/close", h.CloseSheet)
	r.Post("/api/retakes/{id}/sheet/reopen", h.ReopenSheet)
	return r
}

// ── GetSheet ─────────────────────────────────────────────────────────────────

func TestStatement_GetSheet_200(t *testing.T) {
	rid := uuid.New()
	stub := &statementServiceStub{
		getSheetFn: func(_ context.Context, got uuid.UUID) (queries.StatementSheet, error) {
			require.Equal(t, rid, got)
			return queries.StatementSheet{RetakeID: pgutil.PgUUID(rid), Status: "open"}, nil
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/retakes/"+rid.String()+"/sheet", nil), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "open", body["status"])
	require.Equal(t, rid.String(), body["retake_id"])
}

func TestStatement_GetSheet_404(t *testing.T) {
	stub := &statementServiceStub{
		getSheetFn: func(context.Context, uuid.UUID) (queries.StatementSheet, error) {
			return queries.StatementSheet{}, statement.ErrRetakeNotFound
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/retakes/"+uuid.New().String()+"/sheet", nil), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestStatement_GetSheet_400_BadUUID(t *testing.T) {
	stub := &statementServiceStub{
		getSheetFn: func(context.Context, uuid.UUID) (queries.StatementSheet, error) {
			t.Fatal("сервис не должен вызываться при битом UUID")
			return queries.StatementSheet{}, nil
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/retakes/not-a-uuid/sheet", nil), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// ── SaveDraftGrade ───────────────────────────────────────────────────────────

func TestStatement_SaveDraftGrade_204(t *testing.T) {
	stub := &statementServiceStub{
		saveDraftFn: func(_ context.Context, _, _ uuid.UUID, grade int32, _ uuid.UUID) error {
			require.EqualValues(t, 5, grade)
			return nil
		},
	}
	body, _ := json.Marshal(map[string]any{"grade": 5})
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPatch,
		"/api/retakes/"+uuid.New().String()+"/sheet/grades/"+uuid.New().String(),
		bytes.NewReader(body)), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestStatement_SaveDraftGrade_409_SheetClosed(t *testing.T) {
	stub := &statementServiceStub{
		saveDraftFn: func(context.Context, uuid.UUID, uuid.UUID, int32, uuid.UUID) error {
			return statement.ErrSheetClosed
		},
	}
	body, _ := json.Marshal(map[string]any{"grade": 4})
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPatch,
		"/api/retakes/"+uuid.New().String()+"/sheet/grades/"+uuid.New().String(),
		bytes.NewReader(body)), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusConflict, rr.Code)
}

func TestStatement_SaveDraftGrade_400_InvalidGrade(t *testing.T) {
	stub := &statementServiceStub{
		saveDraftFn: func(context.Context, uuid.UUID, uuid.UUID, int32, uuid.UUID) error {
			return statement.ErrInvalidGrade
		},
	}
	body, _ := json.Marshal(map[string]any{"grade": 1})
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPatch,
		"/api/retakes/"+uuid.New().String()+"/sheet/grades/"+uuid.New().String(),
		bytes.NewReader(body)), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestStatement_SaveDraftGrade_404_Participant(t *testing.T) {
	stub := &statementServiceStub{
		saveDraftFn: func(context.Context, uuid.UUID, uuid.UUID, int32, uuid.UUID) error {
			return statement.ErrParticipantNotFound
		},
	}
	body, _ := json.Marshal(map[string]any{"grade": 4})
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPatch,
		"/api/retakes/"+uuid.New().String()+"/sheet/grades/"+uuid.New().String(),
		bytes.NewReader(body)), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestStatement_SaveDraftGrade_400_BadJSON(t *testing.T) {
	stub := &statementServiceStub{
		saveDraftFn: func(context.Context, uuid.UUID, uuid.UUID, int32, uuid.UUID) error {
			t.Fatal("сервис не должен вызываться при битом JSON")
			return nil
		},
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPatch,
		"/api/retakes/"+uuid.New().String()+"/sheet/grades/"+uuid.New().String(),
		bytes.NewReader([]byte("{не json"))), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestStatement_SaveDraftGrade_401_NoAuth(t *testing.T) {
	stub := &statementServiceStub{}
	body, _ := json.Marshal(map[string]any{"grade": 4})
	rr := httptest.NewRecorder()
	// БЕЗ withUser → нет userID в контексте.
	req := httptest.NewRequest(http.MethodPatch,
		"/api/retakes/"+uuid.New().String()+"/sheet/grades/"+uuid.New().String(),
		bytes.NewReader(body))
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ── CloseSheet ───────────────────────────────────────────────────────────────

func TestStatement_CloseSheet_204(t *testing.T) {
	stub := &statementServiceStub{
		closeSheetFn: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost,
		"/api/retakes/"+uuid.New().String()+"/sheet/close", nil), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestStatement_CloseSheet_409_AlreadyClosed(t *testing.T) {
	stub := &statementServiceStub{
		closeSheetFn: func(context.Context, uuid.UUID, uuid.UUID) error { return statement.ErrSheetClosed },
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost,
		"/api/retakes/"+uuid.New().String()+"/sheet/close", nil), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusConflict, rr.Code)
}

// ── ReopenSheet ──────────────────────────────────────────────────────────────

func TestStatement_ReopenSheet_204(t *testing.T) {
	stub := &statementServiceStub{
		reopenFn: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost,
		"/api/retakes/"+uuid.New().String()+"/sheet/reopen", nil), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestStatement_ReopenSheet_409_AlreadyOpen(t *testing.T) {
	stub := &statementServiceStub{
		reopenFn: func(context.Context, uuid.UUID, uuid.UUID) error { return statement.ErrSheetAlreadyOpen },
	}
	rr := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost,
		"/api/retakes/"+uuid.New().String()+"/sheet/reopen", nil), uuid.New())
	statementFixture(handler.NewStatementHandler(stub)).ServeHTTP(rr, req)
	require.Equal(t, http.StatusConflict, rr.Code)
}
