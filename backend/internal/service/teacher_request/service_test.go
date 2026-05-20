package teacher_request_test

import (
	"context"
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
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/rbac"
	teacherrequest "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/teacher_request"
)

type fixture struct {
	store *repo.Store
	rbac  *rbac.Service
	svc   *teacherrequest.Service
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
	rbacSvc := rbac.New(store)
	svc := teacherrequest.New(store, rbacSvc)
	return &fixture{store: store, rbac: rbacSvc, svc: svc}
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
		_, _ = store.Pool().Exec(ctx, "DELETE FROM users WHERE id = $1", u.ID)
	})
	return pgutil.UUID(u.ID)
}

func attachRole(t *testing.T, store *repo.Store, userID uuid.UUID, slug string) {
	t.Helper()
	ctx := context.Background()
	role, err := store.GetRoleBySlug(ctx, slug)
	require.NoError(t, err)
	_, err = store.AttachRoleToUser(ctx, queries.AttachRoleToUserParams{
		UserID:    pgutil.PgUUID(userID),
		RoleID:    role.ID,
		CreatedBy: pgutil.PgUUID(userID),
	})
	require.NoError(t, err)
}

func TestTeacherRequest_Create_Success(t *testing.T) {
	f := setup(t)
	student := seedUser(t, f.store, "tr-create")
	ctx := context.Background()

	reason := "хочу преподавать математику"
	req, err := f.svc.Create(ctx, student, &reason)
	require.NoError(t, err)
	require.Equal(t, "pending", req.Status)
	require.Equal(t, student, pgutil.UUID(req.RequestedBy))
	require.NotEqual(t, uuid.Nil, pgutil.UUID(req.ID))
}

func TestTeacherRequest_Create_AlreadyPending(t *testing.T) {
	// Нельзя подать вторую заявку пока первая ждёт рассмотрения.
	f := setup(t)
	student := seedUser(t, f.store, "tr-dup")
	ctx := context.Background()

	_, err := f.svc.Create(ctx, student, nil)
	require.NoError(t, err)

	_, err = f.svc.Create(ctx, student, nil)
	require.ErrorIs(t, err, teacherrequest.ErrAlreadyPending)
}

func TestTeacherRequest_Approve_AssignsTeacherRole(t *testing.T) {
	// При одобрении: статус → approved и роль teacher выдана в одной tx.
	f := setup(t)
	dean := seedUser(t, f.store, "tr-app-dean")
	student := seedUser(t, f.store, "tr-app-student")
	attachRole(t, f.store, dean, "dean")
	ctx := context.Background()

	req, err := f.svc.Create(ctx, student, nil)
	require.NoError(t, err)

	approved, err := f.svc.Approve(ctx, dean, pgutil.UUID(req.ID), nil)
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Status)
	require.True(t, approved.ReviewedBy.Valid)

	// Роль teacher должна появиться у пользователя.
	roles, err := f.store.ListRolesForUser(ctx, pgutil.PgUUID(student))
	require.NoError(t, err)
	slugs := make([]string, 0, len(roles))
	for _, r := range roles {
		slugs = append(slugs, r.Slug)
	}
	require.Contains(t, slugs, "teacher", "после одобрения студент должен получить роль teacher")
}

func TestTeacherRequest_Reject_Success(t *testing.T) {
	f := setup(t)
	dean := seedUser(t, f.store, "tr-rej-dean")
	student := seedUser(t, f.store, "tr-rej-student")
	attachRole(t, f.store, dean, "dean")
	ctx := context.Background()

	req, err := f.svc.Create(ctx, student, nil)
	require.NoError(t, err)

	rejected, err := f.svc.Reject(ctx, dean, pgutil.UUID(req.ID), "нет свободных мест")
	require.NoError(t, err)
	require.Equal(t, "rejected", rejected.Status)
	require.NotNil(t, rejected.DecisionReason)
	require.Equal(t, "нет свободных мест", *rejected.DecisionReason)
}

func TestTeacherRequest_Reject_ReasonRequired(t *testing.T) {
	// Отклонить без причины нельзя — сервис должен вернуть ErrReasonRequired.
	f := setup(t)
	dean := seedUser(t, f.store, "tr-noreason-dean")
	student := seedUser(t, f.store, "tr-noreason-student")
	attachRole(t, f.store, dean, "dean")
	ctx := context.Background()

	req, err := f.svc.Create(ctx, student, nil)
	require.NoError(t, err)

	_, err = f.svc.Reject(ctx, dean, pgutil.UUID(req.ID), "")
	require.ErrorIs(t, err, teacherrequest.ErrReasonRequired)
}

func TestTeacherRequest_Approve_NotFound(t *testing.T) {
	f := setup(t)
	dean := seedUser(t, f.store, "tr-notfound-dean")
	attachRole(t, f.store, dean, "dean")

	_, err := f.svc.Approve(context.Background(), dean, uuid.New(), nil)
	require.ErrorIs(t, err, teacherrequest.ErrRequestNotFound)
}

func TestTeacherRequest_Approve_AlreadyReviewed(t *testing.T) {
	// Одобрить уже рассмотренную заявку нельзя.
	f := setup(t)
	dean := seedUser(t, f.store, "tr-reviewed-dean")
	student := seedUser(t, f.store, "tr-reviewed-student")
	attachRole(t, f.store, dean, "dean")
	ctx := context.Background()

	req, err := f.svc.Create(ctx, student, nil)
	require.NoError(t, err)

	_, err = f.svc.Approve(ctx, dean, pgutil.UUID(req.ID), nil)
	require.NoError(t, err)

	// Повторная попытка одобрить — должна вернуть ErrNotPending.
	_, err = f.svc.Approve(ctx, dean, pgutil.UUID(req.ID), nil)
	require.ErrorIs(t, err, teacherrequest.ErrNotPending)
}