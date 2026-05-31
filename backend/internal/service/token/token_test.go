package token_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/token"
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

// secret — 32-байтовый ключ для тестов, чтобы не задавать через env.
var secret = []byte("test-secret-must-be-at-least-32-bytes!!")

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

// seedUser создаёт уникального пользователя для теста и возвращает его id.
// Cleanup чистит запись после теста — каскад уберёт связанные токены.
func seedUser(t *testing.T, s *repo.Store, prefix string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	email := strings.ToLower(prefix) + "+" + time.Now().Format("150405.000000") + "@test.local"
	u, err := s.CreateUser(ctx, queries.CreateUserParams{
		Email:        email,
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "T",
		LastName:     "U",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = repo.CleanupUser(ctx, s.Pool(), u.ID)
	})
	return pgutil.UUID(u.ID)
}

func TestService_Issue_ReturnsValidPair(t *testing.T) {
	s := testStore(t)
	svc := token.New(s, secret, 15*time.Minute, 7*24*time.Hour)
	userID := seedUser(t, s, "issue")

	pair, err := svc.Issue(context.Background(), userID, "127.0.0.1")
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
	require.True(t, pair.AccessExpires.After(time.Now()))
	require.True(t, pair.RefreshExpires.After(pair.AccessExpires))
	require.NotEqual(t, pair.AccessToken, pair.RefreshToken)
}

func TestService_Validate_AcceptsFreshToken(t *testing.T) {
	s := testStore(t)
	svc := token.New(s, secret, 15*time.Minute, 7*24*time.Hour)
	userID := seedUser(t, s, "validate")

	pair, err := svc.Issue(context.Background(), userID, "")
	require.NoError(t, err)

	gotUserID, gotTokenID, err := svc.Validate(context.Background(), pair.AccessToken)
	require.NoError(t, err)
	require.Equal(t, userID, gotUserID)
	require.Equal(t, pair.AccessTokenID, gotTokenID)
}

func TestService_Validate_RejectsRevoked(t *testing.T) {
	s := testStore(t)
	svc := token.New(s, secret, 15*time.Minute, 7*24*time.Hour)
	userID := seedUser(t, s, "revoked")

	pair, err := svc.Issue(context.Background(), userID, "")
	require.NoError(t, err)
	require.NoError(t, svc.Revoke(context.Background(), pair.AccessTokenID))

	_, _, err = svc.Validate(context.Background(), pair.AccessToken)
	require.ErrorIs(t, err, token.ErrTokenRevoked)
}

func TestService_Validate_RejectsTampered(t *testing.T) {
	s := testStore(t)
	svc := token.New(s, secret, 15*time.Minute, 7*24*time.Hour)
	userID := seedUser(t, s, "tampered")

	pair, err := svc.Issue(context.Background(), userID, "")
	require.NoError(t, err)

	// Меняем один символ в подписи (последний после точки) — JWT
	// должен перестать проходить ParseWithClaims.
	bad := pair.AccessToken[:len(pair.AccessToken)-1] + "X"
	_, _, err = svc.Validate(context.Background(), bad)
	require.Error(t, err)
	// Может быть ErrTokenInvalid; точный sentinel зависит от того, какую
	// ошибку JWT-парсер вернёт. Любой из этих — ожидаемый.
	require.True(t,
		errors.Is(err, token.ErrTokenInvalid) || errors.Is(err, token.ErrTokenRevoked),
		"ожидалось ErrTokenInvalid/Revoked, получили %v", err)
}

func TestService_Rotate_HappyPath(t *testing.T) {
	s := testStore(t)
	svc := token.New(s, secret, 15*time.Minute, 7*24*time.Hour)
	userID := seedUser(t, s, "rotate")
	ctx := context.Background()

	first, err := svc.Issue(ctx, userID, "")
	require.NoError(t, err)

	second, err := svc.Rotate(ctx, first.RefreshToken, "")
	require.NoError(t, err)
	require.NotEqual(t, first.AccessToken, second.AccessToken)
	require.NotEqual(t, first.RefreshToken, second.RefreshToken)
	require.NotEqual(t, first.AccessTokenID, second.AccessTokenID)

	// Старый access после ротации должен быть отозван.
	_, _, err = svc.Validate(ctx, first.AccessToken)
	require.ErrorIs(t, err, token.ErrTokenRevoked)

	// Новая пара рабочая.
	gotUserID, _, err := svc.Validate(ctx, second.AccessToken)
	require.NoError(t, err)
	require.Equal(t, userID, gotUserID)
}

func TestService_Rotate_ReplayDetectionRevokesAll(t *testing.T) {
	s := testStore(t)
	svc := token.New(s, secret, 15*time.Minute, 7*24*time.Hour)
	userID := seedUser(t, s, "replay")
	ctx := context.Background()

	// Первая выдача и ротация — корректная цепочка.
	first, err := svc.Issue(ctx, userID, "")
	require.NoError(t, err)
	second, err := svc.Rotate(ctx, first.RefreshToken, "")
	require.NoError(t, err)

	// Параллельно у пользователя выдан ещё один сеанс — у атакующего
	// должны быть отозваны и этот тоже, потому что в реальности мы
	// не знаем, какой именно был украден.
	parallel, err := svc.Issue(ctx, userID, "")
	require.NoError(t, err)

	// Повторное использование старого (уже использованного) refresh —
	// это replay. Сервис должен ответить ErrRefreshReplay и отозвать
	// все access-токены пользователя.
	_, err = svc.Rotate(ctx, first.RefreshToken, "")
	require.ErrorIs(t, err, token.ErrRefreshReplay)

	_, _, err = svc.Validate(ctx, second.AccessToken)
	require.ErrorIs(t, err, token.ErrTokenRevoked, "ротированный access тоже должен быть ревокнут")
	_, _, err = svc.Validate(ctx, parallel.AccessToken)
	require.ErrorIs(t, err, token.ErrTokenRevoked, "параллельная сессия тоже должна быть закрыта")
}

func TestService_RevokeAll_ClosesAllSessions(t *testing.T) {
	s := testStore(t)
	svc := token.New(s, secret, 15*time.Minute, 7*24*time.Hour)
	userID := seedUser(t, s, "revokeall")
	ctx := context.Background()

	a, err := svc.Issue(ctx, userID, "")
	require.NoError(t, err)
	b, err := svc.Issue(ctx, userID, "")
	require.NoError(t, err)

	require.NoError(t, svc.RevokeAll(ctx, userID))

	_, _, err = svc.Validate(ctx, a.AccessToken)
	require.ErrorIs(t, err, token.ErrTokenRevoked)
	_, _, err = svc.Validate(ctx, b.AccessToken)
	require.ErrorIs(t, err, token.ErrTokenRevoked)
}

// TestService_Rotate_ParallelCallsExactlyOneSucceeds: два параллельных
// Rotate с одним refresh-токеном. Раньше оба могли пройти проверку
// is_used=FALSE и оба получить новые пары (race). После фикса с
// SELECT FOR UPDATE второй вызов должен дождаться коммита первого,
// увидеть is_used=TRUE и вернуть ErrRefreshReplay.
func TestService_Rotate_ParallelCallsExactlyOneSucceeds(t *testing.T) {
	s := testStore(t)
	svc := token.New(s, secret, 15*time.Minute, 7*24*time.Hour)
	userID := seedUser(t, s, "race")
	ctx := context.Background()

	pair, err := svc.Issue(ctx, userID, "")
	require.NoError(t, err)

	type result struct {
		pair *token.Pair
		err  error
	}
	results := make(chan result, 2)
	start := make(chan struct{})

	for range 2 {
		go func() {
			<-start // дать обоим стартовать одновременно
			p, err := svc.Rotate(ctx, pair.RefreshToken, "")
			results <- result{pair: p, err: err}
		}()
	}
	close(start)

	r1 := <-results
	r2 := <-results

	// Ровно один успешный (новая пара), ровно один replay-failure.
	successes := 0
	replays := 0
	for _, r := range []result{r1, r2} {
		switch {
		case r.err == nil && r.pair != nil:
			successes++
		case errors.Is(r.err, token.ErrRefreshReplay):
			replays++
		default:
			t.Fatalf("неожиданный результат: pair=%v err=%v", r.pair, r.err)
		}
	}
	require.Equal(t, 1, successes, "ровно один параллельный Rotate должен преуспеть")
	require.Equal(t, 1, replays, "второй Rotate должен получить ErrRefreshReplay")

	// Побочный эффект replay-detection: все access-токены пользователя
	// должны быть ревокированы (это защита от кражи refresh).
	// Даже свежевыданная пара из successful Rotate уже ревокнута.
	for _, r := range []result{r1, r2} {
		if r.err == nil {
			_, _, validateErr := svc.Validate(ctx, r.pair.AccessToken)
			require.ErrorIs(t, validateErr, token.ErrTokenRevoked,
				"replay-detection должен ревокнуть все токены пользователя")
		}
	}
}
