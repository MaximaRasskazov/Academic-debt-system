package audit_test

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
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
)

// TestMain прогоняет тесты пакета, затем страховочно дочищает тестовый
// мусор (@test.local). Per-test cleanup может не сработать при гонках
// параллельных пакетов на общей БД — TestMain гарантирует 0 остатка.
func TestMain(m *testing.M) {
	code := m.Run()
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		if pool, err := pgxpool.New(context.Background(), dsn); err == nil {
			_ = repo.CleanupAllTestUsers(context.Background(), pool)
			pool.Close()
		}
	}
	os.Exit(code)
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

// seedUser нужен потому что audit_log.actor_id FK на users.
func seedUser(t *testing.T, s *repo.Store) uuid.UUID {
	t.Helper()
	email := "audit+" + time.Now().Format("150405.000000") + "@test.local"
	u, err := s.CreateUser(context.Background(), queries.CreateUserParams{
		Email:        email,
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "T",
		LastName:     "A",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = repo.CleanupUser(context.Background(), s.Pool(), u.ID)
	})
	return pgutil.UUID(u.ID)
}

func TestAudit_Log_WritesEntry(t *testing.T) {
	s := testStore(t)
	svc := audit.New(s)
	actor := seedUser(t, s)
	target := uuid.New().String()
	ctx := context.Background()

	err := svc.Log(ctx, audit.Event{
		ActorID:    actor,
		Action:     "test.action",
		TargetType: audit.TargetTypeUser,
		TargetID:   target,
		Details:    map[string]any{"foo": "bar", "n": 42},
		IPAddress:  "127.0.0.1",
	})
	require.NoError(t, err)

	rows, err := s.ListAuditByActor(ctx, queries.ListAuditByActorParams{
		ActorID: pgutil.PgUUID(actor),
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	got := rows[0]
	require.Equal(t, "test.action", got.Action)
	require.Equal(t, audit.TargetTypeUser, got.TargetType)
	require.NotNil(t, got.TargetID)
	require.Equal(t, target, *got.TargetID)
	require.NotNil(t, got.IpAddress)
	require.Equal(t, "127.0.0.1", *got.IpAddress)

	var details map[string]any
	require.NoError(t, json.Unmarshal(got.Details, &details))
	require.Equal(t, "bar", details["foo"])
	require.EqualValues(t, 42, details["n"])
}

func TestAudit_LogTx_RolledBackOnError(t *testing.T) {
	// Ключевой инвариант: audit-запись должна жить ВНУТРИ той же
	// транзакции, что и основная мутация. Если callback вернёт
	// ошибку — audit тоже не появится.
	s := testStore(t)
	svc := audit.New(s)
	actor := seedUser(t, s)
	ctx := context.Background()

	sentinel := error(nil) // заполним внутри callback'а
	err := s.RunInTx(ctx, func(q *queries.Queries) error {
		if err := svc.LogTx(ctx, q, audit.Event{
			ActorID:    actor,
			Action:     "test.rolled_back",
			TargetType: audit.TargetTypeUser,
		}); err != nil {
			return err
		}
		sentinel = errAbort
		return sentinel
	})
	require.ErrorIs(t, err, errAbort)

	rows, err := s.ListAuditByActor(ctx, queries.ListAuditByActorParams{
		ActorID: pgutil.PgUUID(actor),
		Limit:   100,
		Offset:  0,
	})
	require.NoError(t, err)
	for _, r := range rows {
		require.NotEqual(t, "test.rolled_back", r.Action,
			"audit-запись должна была откатиться вместе с транзакцией")
	}
}

func TestAudit_LogTx_CommitsWithMainMutation(t *testing.T) {
	s := testStore(t)
	svc := audit.New(s)
	actor := seedUser(t, s)
	ctx := context.Background()

	err := s.RunInTx(ctx, func(q *queries.Queries) error {
		return svc.LogTx(ctx, q, audit.Event{
			ActorID:    actor,
			Action:     "test.committed",
			TargetType: audit.TargetTypeUser,
			TargetID:   actor.String(),
			Details:    map[string]any{"ok": true},
		})
	})
	require.NoError(t, err)

	rows, err := s.ListAuditByActor(ctx, queries.ListAuditByActorParams{
		ActorID: pgutil.PgUUID(actor),
		Limit:   100,
		Offset:  0,
	})
	require.NoError(t, err)

	var found bool
	for _, r := range rows {
		if r.Action == "test.committed" {
			found = true
			break
		}
	}
	require.True(t, found, "после успешной транзакции audit-запись должна быть видна")
}

func TestAudit_Log_RejectsMissingFields(t *testing.T) {
	s := testStore(t)
	svc := audit.New(s)
	ctx := context.Background()

	err := svc.Log(ctx, audit.Event{TargetType: "x"})
	require.Error(t, err, "action обязателен")

	err = svc.Log(ctx, audit.Event{Action: "x"})
	require.Error(t, err, "target_type обязателен")
}

// errAbort — sentinel для tests, чтобы возвращать ошибку из tx-callback.
var errAbort = errAbortType("test-abort")

type errAbortType string

func (e errAbortType) Error() string { return string(e) }
