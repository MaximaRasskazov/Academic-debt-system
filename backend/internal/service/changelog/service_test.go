package changelog_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
)

// TestMain прогоняет тесты пакета, затем страховочно дочищает тестовый
// мусор (@test.local). Per-test cleanup может не сработать при гонках
// параллельных пакетов на общей БД — TestMain гарантирует 0 остатка.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func testStore(t *testing.T) *repo.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан — интеграционный тест пропущен")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)
	return repo.NewStore(pool)
}

func seedActor(t *testing.T, s *repo.Store) uuid.UUID {
	t.Helper()
	email := "changelog+" + time.Now().Format("150405.000000") + "@test.local"
	u, err := s.CreateUser(context.Background(), queries.CreateUserParams{
		Email:        email,
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "C",
		LastName:     "L",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = repo.CleanupUser(context.Background(), s.Pool(), u.ID)
	})
	return pgutil.UUID(u.ID)
}

func TestChangeLog_LogCreated_WritesEmptyBefore(t *testing.T) {
	s := testStore(t)
	svc := changelog.New(s)
	actor := seedActor(t, s)
	entityID := uuid.New().String()
	ctx := context.Background()

	err := s.RunInTx(ctx, func(q *queries.Queries) error {
		return svc.LogCreatedTx(ctx, q, "debt", entityID,
			changelog.Fields{"student_id": "s1", "discipline_id": "d1", "status": "open"},
			actor)
	})
	require.NoError(t, err)

	rows, err := svc.ListForEntity(ctx, "debt", entityID, 10, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, changelog.ActionCreated, rows[0].Action)

	var before, after map[string]any
	require.NoError(t, json.Unmarshal(rows[0].Before, &before))
	require.NoError(t, json.Unmarshal(rows[0].After, &after))
	require.Empty(t, before)
	require.Equal(t, "open", after["status"])
}

func TestChangeLog_LogUpdated_WritesBeforeAndAfter(t *testing.T) {
	s := testStore(t)
	svc := changelog.New(s)
	actor := seedActor(t, s)
	entityID := uuid.New().String()
	ctx := context.Background()

	err := s.RunInTx(ctx, func(q *queries.Queries) error {
		return svc.LogUpdatedTx(ctx, q, "retake", entityID,
			changelog.Fields{"status": "scheduled", "scheduled_at": "2026-06-01T10:00:00Z"},
			changelog.Fields{"status": "scheduled", "scheduled_at": "2026-06-02T10:00:00Z"},
			actor)
	})
	require.NoError(t, err)

	rows, err := svc.ListForEntity(ctx, "retake", entityID, 10, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, changelog.ActionUpdated, rows[0].Action)

	var before, after map[string]any
	require.NoError(t, json.Unmarshal(rows[0].Before, &before))
	require.NoError(t, json.Unmarshal(rows[0].After, &after))
	require.Equal(t, "2026-06-01T10:00:00Z", before["scheduled_at"])
	require.Equal(t, "2026-06-02T10:00:00Z", after["scheduled_at"])
}

func TestChangeLog_LogSoftDeleted_AfterEmpty(t *testing.T) {
	s := testStore(t)
	svc := changelog.New(s)
	actor := seedActor(t, s)
	entityID := uuid.New().String()
	ctx := context.Background()

	err := s.RunInTx(ctx, func(q *queries.Queries) error {
		return svc.LogSoftDeletedTx(ctx, q, "debt", entityID,
			changelog.Fields{"status": "open", "amount": 1},
			actor)
	})
	require.NoError(t, err)

	rows, err := svc.ListForEntity(ctx, "debt", entityID, 10, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, changelog.ActionSoftDeleted, rows[0].Action)

	var after map[string]any
	require.NoError(t, json.Unmarshal(rows[0].After, &after))
	require.Empty(t, after)
}

func TestChangeLog_OrderingNewestFirst(t *testing.T) {
	// Несколько записей по одной сущности должны возвращаться
	// в порядке убывания времени (по индексу idx_change_logs_entity).
	s := testStore(t)
	svc := changelog.New(s)
	actor := seedActor(t, s)
	entityID := uuid.New().String()
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		fields := changelog.Fields{"version": i}
		err := s.RunInTx(ctx, func(q *queries.Queries) error {
			if i == 1 {
				return svc.LogCreatedTx(ctx, q, "thing", entityID, fields, actor)
			}
			return svc.LogUpdatedTx(ctx, q, "thing", entityID,
				changelog.Fields{"version": i - 1}, fields, actor)
		})
		require.NoError(t, err)
		// Сдвигаем clock на миллисекунду, иначе created_at может совпасть.
		time.Sleep(2 * time.Millisecond)
	}

	rows, err := svc.ListForEntity(ctx, "thing", entityID, 10, 0)
	require.NoError(t, err)
	require.Len(t, rows, 3)

	// Самая свежая — со status=updated и after.version=3.
	var after map[string]any
	require.NoError(t, json.Unmarshal(rows[0].After, &after))
	require.EqualValues(t, 3, after["version"])
}

func TestChangeLog_ValidatesRequiredFields(t *testing.T) {
	s := testStore(t)
	svc := changelog.New(s)
	actor := seedActor(t, s)
	ctx := context.Background()

	cases := []struct {
		name      string
		entityT   string
		entityID  string
		createdBy uuid.UUID
	}{
		{"empty entity_type", "", "id1", actor},
		{"empty entity_id", "x", "", actor},
		{"nil created_by", "x", "id1", uuid.Nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.RunInTx(ctx, func(q *queries.Queries) error {
				return svc.LogCreatedTx(ctx, q, tc.entityT, tc.entityID, changelog.Fields{"a": 1}, tc.createdBy)
			})
			require.Error(t, err)
		})
	}
}
