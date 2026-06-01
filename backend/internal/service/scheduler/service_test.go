package scheduler_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/scheduler"
)

// TestMain прогоняет тесты пакета, затем страховочно дочищает тестовый
// мусор (@test.local). Per-test cleanup может не сработать при гонках
// параллельных пакетов на общей БД — TestMain гарантирует 0 остатка.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

type fixture struct {
	store *repo.Store
	disc  *discipline.Service
	svc   *scheduler.Service
}

func setup(t *testing.T) *fixture {
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

	store := repo.NewStore(pool)
	auditSvc := audit.New(store)
	disc := discipline.New(store, auditSvc, changelog.New(store))
	// Короткий interval для теста — но Tick мы дёргаем напрямую,
	// поэтому реально interval тут не критичен.
	return &fixture{
		store: store, disc: disc,
		svc: scheduler.New(store, auditSvc, 10*time.Millisecond),
	}
}

func seedUser(t *testing.T, store *repo.Store, prefix string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	email := strings.ToLower(prefix) + "+" + time.Now().Format("150405.000000000") + "@test.local"
	u, err := store.CreateUser(ctx, queries.CreateUserParams{
		Email:        email,
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "T",
		LastName:     "U",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = repo.CleanupUser(ctx, store.Pool(), u.ID)
	})
	return pgutil.UUID(u.ID)
}

// seedDiscipline создаёт дисциплину (нужна для FK retake.discipline_id).
func seedDiscipline(t *testing.T, f *fixture, prefix string, owner uuid.UUID) uuid.UUID {
	t.Helper()
	suf := time.Now().Format("150405.000000000")
	d, err := f.disc.Create(context.Background(), discipline.CreateInput{
		Name: prefix + "-" + suf, Code: prefix + "-" + suf,
	}, owner)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM disciplines WHERE id = $1", d.ID)
	})
	return pgutil.UUID(d.ID)
}

// createRetakeWithStatus вставляет пересдачу напрямую через sqlc с
// нужными временем и статусом. UpdateRetakeSchedule + ручной UPDATE
// для статуса — sqlc-метод смены статуса без сервиса нам не нужен.
// Здесь идём напрямую в БД через store.Pool().Exec — это test-only
// способ симулировать "пора стартовать" / "пора заканчивать".
func createRetakeWithStatus(t *testing.T, f *fixture, disciplineID uuid.UUID, scheduledAt time.Time, durationMin int32, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	dean := seedUser(t, f.store, "dean-r-"+status)

	row, err := f.store.CreateRetake(ctx, queries.CreateRetakeParams{
		DisciplineID:    pgutil.PgUUID(disciplineID),
		Kind:            "regular",
		MinTeachers:     1,
		Building:        "А",
		Room:            "100",
		ScheduledAt:     pgtype.Timestamptz{Time: scheduledAt, Valid: true},
		DurationMinutes: durationMin,
		CreatedBy:       pgutil.PgUUID(dean),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(ctx, "DELETE FROM retakes WHERE id = $1", row.ID)
	})

	if status != "scheduled" {
		// БД-CHECK retakes_completed_consistency требует, чтобы
		// completed_at был заполнен ⇔ status='completed'. Поэтому
		// при засеивании "completed" мы выставляем completed_at в
		// том же UPDATE'е.
		if status == "completed" {
			_, err := f.store.Pool().Exec(ctx,
				"UPDATE retakes SET status = $1, completed_at = NOW() WHERE id = $2",
				status, row.ID)
			require.NoError(t, err)
		} else {
			_, err := f.store.Pool().Exec(ctx,
				"UPDATE retakes SET status = $1 WHERE id = $2", status, row.ID)
			require.NoError(t, err)
		}
	}
	return pgutil.UUID(row.ID)
}

// statusOf достаёт текущий статус пересдачи из БД.
func statusOf(t *testing.T, store *repo.Store, id uuid.UUID) string {
	t.Helper()
	row, err := store.GetRetakeByID(context.Background(), pgutil.PgUUID(id))
	require.NoError(t, err)
	return row.Status
}

func TestScheduler_Tick_TransitionsScheduledToInProgress(t *testing.T) {
	f := setup(t)
	disc := seedDiscipline(t, f, "SCH", seedUser(t, f.store, "owner-sc"))

	// Пересдача стартовала 5 минут назад, длится 60 мин — должна
	// перейти в in_progress, но ещё не в completed (завершение через
	// 55 минут).
	id := createRetakeWithStatus(t, f, disc, time.Now().Add(-5*time.Minute), 60, "scheduled")

	f.svc.Tick(context.Background())

	require.Equal(t, "in_progress", statusOf(t, f.store, id))
}

func TestScheduler_Tick_TransitionsInProgressToCompleted(t *testing.T) {
	f := setup(t)
	disc := seedDiscipline(t, f, "SCH", seedUser(t, f.store, "owner-fn"))

	// Пересдача стартовала 3 часа назад, длилась 60 мин — давно пора
	// в completed. Сначала ставим in_progress, чтобы протестировать
	// именно завершение (а не объединённый scheduled → completed).
	id := createRetakeWithStatus(t, f, disc, time.Now().Add(-3*time.Hour), 60, "in_progress")

	f.svc.Tick(context.Background())

	row, err := f.store.GetRetakeByID(context.Background(), pgutil.PgUUID(id))
	require.NoError(t, err)
	require.Equal(t, "completed", row.Status)
	require.True(t, row.CompletedAt.Valid, "completed_at должен быть заполнен (БД-CHECK)")
}

func TestScheduler_Tick_NoOpWhenNoReadyRetakes(t *testing.T) {
	f := setup(t)
	disc := seedDiscipline(t, f, "SCH", seedUser(t, f.store, "owner-no"))

	// Пересдача в будущем — шедулер её не трогает.
	id := createRetakeWithStatus(t, f, disc, time.Now().Add(24*time.Hour), 60, "scheduled")

	f.svc.Tick(context.Background())

	require.Equal(t, "scheduled", statusOf(t, f.store, id),
		"будущая пересдача должна оставаться scheduled")
}

func TestScheduler_Tick_Idempotent(t *testing.T) {
	f := setup(t)
	disc := seedDiscipline(t, f, "SCH", seedUser(t, f.store, "owner-id"))

	// Уже completed — повторные тики не должны трогать.
	// createRetakeWithStatus сам выставит completed_at для status=completed.
	id := createRetakeWithStatus(t, f, disc, time.Now().Add(-2*time.Hour), 30, "completed")

	for i := 0; i < 3; i++ {
		f.svc.Tick(context.Background())
	}

	require.Equal(t, "completed", statusOf(t, f.store, id),
		"completed-пересдача должна остаться в этом статусе после нескольких тиков")
}

func TestScheduler_Tick_HandlesScheduledPastDueAtomically(t *testing.T) {
	// Сценарий: пересдача в scheduled, время старта прошло, время
	// завершения тоже прошло. Шедулер должен сначала перевести в
	// in_progress, потом в completed (одним тиком — это покрывает
	// случай "сервер был выключен 2 часа, пропустил начало").
	f := setup(t)
	disc := seedDiscipline(t, f, "SCH", seedUser(t, f.store, "owner-catchup"))

	// Час назад начала + 30 мин длительности = должна быть completed.
	id := createRetakeWithStatus(t, f, disc, time.Now().Add(-1*time.Hour), 30, "scheduled")

	f.svc.Tick(context.Background())

	row, err := f.store.GetRetakeByID(context.Background(), pgutil.PgUUID(id))
	require.NoError(t, err)
	require.Equal(t, "completed", row.Status,
		"одна пересдача должна догнаться сразу в completed одним тиком")
	require.True(t, row.CompletedAt.Valid)
}

func TestScheduler_Run_StopsOnContextCancel(t *testing.T) {
	// Run должен корректно завершиться через ctx.Done(). Это тест
	// graceful shutdown — без него main.go не сможет аккуратно
	// останавливать сервис при SIGTERM.
	f := setup(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- f.svc.Run(ctx)
	}()

	// Даём шедулеру стартануть первый тик и сесть на ticker.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler.Run не завершился через 2 секунды после cancel")
	}
}
