package user_test

// Интеграционные тесты ListUsers. Опираемся на сиды dev-аккаунтов
// из 00001_seed_dev_accounts.sql — там 7 учеток (1 admin, 1 dean,
// 2 teacher, 3 student).

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/user"
)

func setup(t *testing.T) *user.Service {
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

	return user.New(repo.NewStore(pool))
}

func TestList_NoFilters_ReturnsTotal(t *testing.T) {
	svc := setup(t)
	res, err := svc.List(context.Background(), user.ListInput{Limit: 50})
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Total, int64(7),
		"в БД должно быть минимум 7 сидовых пользователей (admin/dean/2 teacher/3 student)")
	require.NotEmpty(t, res.Items)
}

func TestList_SeededAccountsHaveRoles(t *testing.T) {
	// Проверяем что СИДОВЫЕ аккаунты находятся через search и у них
	// есть роли. Делаем по одному запросу на email — независимо от того
	// сколько leftover-юзеров накопилось в БД от auth-тестов.
	svc := setup(t)
	for _, email := range []string{
		"admin@academic.local",
		"dean@academic.local",
		"teacher1@academic.local",
		"student1@academic.local",
	} {
		res, err := svc.List(context.Background(), user.ListInput{
			Search: email,
			Limit:  5,
		})
		require.NoError(t, err)
		require.Len(t, res.Items, 1, "сидовый %s должен находиться точным search", email)
		require.NotEmpty(t, res.Items[0].Roles, "сидовый %s должен быть с ролью", email)
	}
}

func TestList_FilterByRoleTeacher(t *testing.T) {
	svc := setup(t)

	res, err := svc.List(context.Background(), user.ListInput{
		RoleSlug: "teacher",
		Limit:    50,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Items), 2, "минимум 2 преподавателя из сидов")
	for _, u := range res.Items {
		// Хотя бы одна роль должна быть teacher (у пользователя может
		// быть несколько ролей, фильтр требует ХОТЯ БЫ teacher).
		hasTeacher := false
		for _, r := range u.Roles {
			if r.Slug == "teacher" {
				hasTeacher = true
				break
			}
		}
		require.True(t, hasTeacher,
			"при фильтре role=teacher в выдаче только teacher'ы, но в %s ролей нет teacher: %+v",
			u.User.Email, u.Roles)
	}
}

func TestList_FilterByRoleStudent(t *testing.T) {
	svc := setup(t)
	res, err := svc.List(context.Background(), user.ListInput{RoleSlug: "student", Limit: 50})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Items), 3, "минимум 3 студента из сидов")
}

func TestList_FilterBySearchEmail(t *testing.T) {
	svc := setup(t)
	res, err := svc.List(context.Background(), user.ListInput{
		Search: "teacher1@academic.local",
		Limit:  10,
	})
	require.NoError(t, err)
	require.Len(t, res.Items, 1, "точный поиск по email должен вернуть одного")
	require.Equal(t, "teacher1@academic.local", res.Items[0].User.Email)
}

func TestList_FilterBySearchLastName_CaseInsensitive(t *testing.T) {
	svc := setup(t)
	// "Преподов" — фамилия teacher1 из сидов
	res, err := svc.List(context.Background(), user.ListInput{
		Search: "преподов",
		Limit:  10,
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Items, "ILIKE должен матчить регистронезависимо")
	require.Contains(t, strings.ToLower(res.Items[0].User.LastName), "преподов")
}

func TestList_FilterByGroupName(t *testing.T) {
	svc := setup(t)
	res, err := svc.List(context.Background(), user.ListInput{
		GroupName: "БСБО-01-22",
		Limit:     10,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Items), 2, "в группе БСБО-01-22 минимум 2 студента")
	for _, u := range res.Items {
		require.NotNil(t, u.User.GroupName)
		require.Equal(t, "БСБО-01-22", *u.User.GroupName)
	}
}

func TestList_Pagination_LimitOffset(t *testing.T) {
	svc := setup(t)

	page1, err := svc.List(context.Background(), user.ListInput{Limit: 2, Offset: 0})
	require.NoError(t, err)
	require.Len(t, page1.Items, 2)

	page2, err := svc.List(context.Background(), user.ListInput{Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.NotEmpty(t, page2.Items)

	// Страницы не должны пересекаться.
	require.NotEqual(t, page1.Items[0].User.ID, page2.Items[0].User.ID)
	require.NotEqual(t, page1.Items[1].User.ID, page2.Items[0].User.ID)

	// Total одинаков на обеих страницах.
	require.Equal(t, page1.Total, page2.Total)
}

func TestList_LimitClamping(t *testing.T) {
	svc := setup(t)

	// 0 → defaultLimit (50)
	zero, err := svc.List(context.Background(), user.ListInput{Limit: 0})
	require.NoError(t, err)
	require.Equal(t, int32(50), zero.Limit, "Limit=0 должен превратиться в 50")

	// > maxLimit → maxLimit (200)
	huge, err := svc.List(context.Background(), user.ListInput{Limit: 9999})
	require.NoError(t, err)
	require.Equal(t, int32(200), huge.Limit, "Limit=9999 должен быть зажат до 200")
}

func TestList_NoMatch_EmptyResult(t *testing.T) {
	svc := setup(t)
	res, err := svc.List(context.Background(), user.ListInput{
		Search: "no-such-user-anywhere-xyz-" + time.Now().Format("150405"),
		Limit:  10,
	})
	require.NoError(t, err)
	require.Empty(t, res.Items)
	require.Equal(t, int64(0), res.Total)
}
