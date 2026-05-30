package handler_test

// Unit-тесты для DirectoryHandler — узкие справочники /api/teachers
// и /api/students/debtors. Используют stub'ы DirectoryUsers/DirectoryDebts,
// БД не нужна. Auth и RBAC-middleware намеренно не применяем — это
// зона ответственности интеграционных проверок (smoke в README).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/user"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/handler"
)

// directoryUsersStub реализует handler.DirectoryUsers.
type directoryUsersStub struct {
	listFn func(ctx context.Context, in user.ListInput) (*user.ListResult, error)
}

func (s *directoryUsersStub) List(ctx context.Context, in user.ListInput) (*user.ListResult, error) {
	return s.listFn(ctx, in)
}

// directoryDebtsStub реализует handler.DirectoryDebts.
type directoryDebtsStub struct {
	listDebtorsFn func(ctx context.Context, disciplineID uuid.UUID) ([]queries.ListDebtorsByDisciplineRow, error)
}

func (s *directoryDebtsStub) ListDebtorsByDiscipline(ctx context.Context, disciplineID uuid.UUID) ([]queries.ListDebtorsByDisciplineRow, error) {
	return s.listDebtorsFn(ctx, disciplineID)
}

func directoryFixture(h *handler.DirectoryHandler) *chi.Mux {
	r := chi.NewMux()
	r.Get("/api/teachers", h.ListTeachers)
	r.Get("/api/students/debtors", h.ListDebtors)
	return r
}

// ── /api/teachers ─────────────────────────────────────────────

func TestDirectory_ListTeachers_OK(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	middle := "Сергеевич"
	stub := &directoryUsersStub{
		listFn: func(_ context.Context, in user.ListInput) (*user.ListResult, error) {
			require.Equal(t, "teacher", in.RoleSlug, "должен фильтровать по role=teacher")
			require.Equal(t, int32(200), in.Limit)
			return &user.ListResult{
				Items: []user.UserWithRoles{
					{User: queries.User{ID: pgutil.PgUUID(id1), FirstName: "Иван", LastName: "Петров", MiddleName: &middle}},
					{User: queries.User{ID: pgutil.PgUUID(id2), FirstName: "Мария", LastName: "Сидорова"}},
				},
				Total:  2,
				Limit:  200,
				Offset: 0,
			}, nil
		},
	}
	h := handler.NewDirectoryHandler(stub, nil)
	r := directoryFixture(h)

	req := httptest.NewRequest(http.MethodGet, "/api/teachers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 2)
	assert.Equal(t, "Петров", resp.Items[0]["last_name"])
	// middle_name есть у первого и опущен у второго (omitempty).
	assert.Equal(t, "Сергеевич", resp.Items[0]["middle_name"])
	_, hasMid := resp.Items[1]["middle_name"]
	assert.False(t, hasMid, "middle_name должен быть omitempty")
	// Никаких email / password_hash / created_at в DTO.
	_, hasEmail := resp.Items[0]["email"]
	assert.False(t, hasEmail, "email НЕ должен экспозиться в TeacherBrief")
}

func TestDirectory_ListTeachers_ServiceError(t *testing.T) {
	stub := &directoryUsersStub{
		listFn: func(context.Context, user.ListInput) (*user.ListResult, error) {
			return nil, errors.New("db down")
		},
	}
	h := handler.NewDirectoryHandler(stub, nil)
	r := directoryFixture(h)

	req := httptest.NewRequest(http.MethodGet, "/api/teachers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal")
}

// ── /api/students/debtors ─────────────────────────────────────

func TestDirectory_ListDebtors_OK(t *testing.T) {
	discID := uuid.New()
	debtID := uuid.New()
	studentID := uuid.New()
	group := "ИС-31"
	stub := &directoryDebtsStub{
		listDebtorsFn: func(_ context.Context, gotDisc uuid.UUID) ([]queries.ListDebtorsByDisciplineRow, error) {
			require.Equal(t, discID, gotDisc, "должен пробросить discipline_id")
			return []queries.ListDebtorsByDisciplineRow{{
				DebtID:    pgutil.PgUUID(debtID),
				StudentID: pgutil.PgUUID(studentID),
				FirstName: "Алексей",
				LastName:  "Иванов",
				GroupName: &group,
			}}, nil
		},
	}
	h := handler.NewDirectoryHandler(nil, stub)
	r := directoryFixture(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/students/debtors?discipline_id="+discID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	assert.Equal(t, debtID.String(), resp.Items[0]["debt_id"])
	assert.Equal(t, studentID.String(), resp.Items[0]["student_id"])
	assert.Equal(t, "ИС-31", resp.Items[0]["group_name"])
}

func TestDirectory_ListDebtors_MissingParam(t *testing.T) {
	h := handler.NewDirectoryHandler(nil, &directoryDebtsStub{})
	r := directoryFixture(h)

	req := httptest.NewRequest(http.MethodGet, "/api/students/debtors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "missing_param")
}

func TestDirectory_ListDebtors_BadUUID(t *testing.T) {
	h := handler.NewDirectoryHandler(nil, &directoryDebtsStub{})
	r := directoryFixture(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/students/debtors?discipline_id=not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_id")
}

func TestDirectory_ListDebtors_ServiceError(t *testing.T) {
	stub := &directoryDebtsStub{
		listDebtorsFn: func(context.Context, uuid.UUID) ([]queries.ListDebtorsByDisciplineRow, error) {
			return nil, errors.New("db down")
		},
	}
	h := handler.NewDirectoryHandler(nil, stub)
	r := directoryFixture(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/students/debtors?discipline_id="+uuid.NewString(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Sanity-check: pgtype.UUID конвертится в uuid.UUID при использовании pgutil.
// Не тест per se, а компиляционный страж — если кто-то меняет утилиты,
// чтоб не сломать сериализацию.
func TestDirectory_PgUUIDRoundtrip(t *testing.T) {
	original := uuid.New()
	pg := pgutil.PgUUID(original)
	require.True(t, pg.Valid)
	require.Equal(t, original, pgutil.UUID(pg))

	// Прямая проверка что pgtype.UUID.Bytes — это [16]byte, как ждёт uuid.
	_ = pgtype.UUID{}
}
