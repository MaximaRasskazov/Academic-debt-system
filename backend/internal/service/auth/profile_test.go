package auth_test

// Интеграционные тесты для UpdateProfile и ChangePassword.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/auth"
)

// registerOne — helper: регистрирует уникального пользователя и
// возвращает uuid + email/password для последующих манипуляций.
// Регистрирует cleanup, чтобы тестовый юзер (и его audit/role записи)
// удалялись после теста и не копились в dev-БД.
func registerOne(t *testing.T, store *repo.Store, svc *auth.Service, prefix string) (id uuid.UUID, email, password string) {
	t.Helper()
	in := uniqueRegisterInput(prefix)
	r, err := svc.Register(context.Background(), in, "127.0.0.1")
	require.NoError(t, err)
	cleanupUser(t, store, r.User.ID)
	return pgutil.UUID(r.User.ID), in.Email, in.Password
}

func TestUpdateProfile_UpdatesOnlyProvidedFields(t *testing.T) {
	store, svc := testServices(t)
	uid, _, _ := registerOne(t, store, svc, "patch-partial")

	// Меняем только last_name. first_name должен остаться как в Register.
	newLast := "Новиков"
	got, err := svc.UpdateProfile(context.Background(), uid,
		auth.UpdateProfileInput{LastName: &newLast})
	require.NoError(t, err)
	require.Equal(t, "Новиков", got.User.LastName)
	require.Equal(t, "Виталий", got.User.FirstName, "first_name не должен меняться при nil")

	// Роли тоже подтянулись — Profile целиком, фронт сразу обновит UI.
	require.NotEmpty(t, got.Roles, "у нового пользователя должна быть как минимум роль student")
}

func TestUpdateProfile_TrimSpaces(t *testing.T) {
	store, svc := testServices(t)
	uid, _, _ := registerOne(t, store, svc, "patch-trim")

	spaced := "   Иванов   "
	got, err := svc.UpdateProfile(context.Background(), uid,
		auth.UpdateProfileInput{LastName: &spaced})
	require.NoError(t, err)
	require.Equal(t, "Иванов", got.User.LastName, "пробелы должны быть отрезаны")
}

func TestUpdateProfile_RejectsEmptyFirstLastName(t *testing.T) {
	store, svc := testServices(t)
	uid, _, _ := registerOne(t, store, svc, "patch-empty")

	empty := "   "
	_, err := svc.UpdateProfile(context.Background(), uid,
		auth.UpdateProfileInput{FirstName: &empty})
	require.Error(t, err, "пустой first_name должен отвергнуть")
}

func TestChangePassword_HappyPath(t *testing.T) {
	store, svc := testServices(t)
	uid, email, oldPwd := registerOne(t, store, svc, "pwd-ok")

	newPwd := "new-password-yo-2026"
	require.NoError(t, svc.ChangePassword(context.Background(), uid, oldPwd, newPwd))

	// Логин со старым паролем больше не работает.
	_, err := svc.Login(context.Background(), email, oldPwd, "127.0.0.1")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)

	// Логин с новым паролем работает.
	res, err := svc.Login(context.Background(), email, newPwd, "127.0.0.1")
	require.NoError(t, err)
	require.NotEmpty(t, res.Pair.AccessToken)
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	store, svc := testServices(t)
	uid, _, _ := registerOne(t, store, svc, "pwd-bad")

	err := svc.ChangePassword(context.Background(), uid, "definitely-not-the-password", "new-password-yo-2026")
	require.ErrorIs(t, err, auth.ErrInvalidPassword)
}

func TestChangePassword_TooShort(t *testing.T) {
	store, svc := testServices(t)
	uid, _, oldPwd := registerOne(t, store, svc, "pwd-short")

	err := svc.ChangePassword(context.Background(), uid, oldPwd, "short")
	require.ErrorIs(t, err, auth.ErrPasswordTooShort)
}

func TestChangePassword_SameAsCurrent(t *testing.T) {
	store, svc := testServices(t)
	uid, _, oldPwd := registerOne(t, store, svc, "pwd-same")

	err := svc.ChangePassword(context.Background(), uid, oldPwd, oldPwd)
	require.ErrorIs(t, err, auth.ErrSamePassword)
}

func TestChangePassword_RevokesAccessTokens(t *testing.T) {
	// Главная гарантия безопасности: после смены пароля старый access-токен
	// помечается revoked в БД. Это защищает от сценария "украли access —
	// пользователь меняет пароль — атакующий продолжает".
	store, svc := testServices(t)
	in := uniqueRegisterInput("pwd-revoke")
	r, err := svc.Register(context.Background(), in, "127.0.0.1")
	require.NoError(t, err)
	uid := pgutil.UUID(r.User.ID)
	oldAccess := r.Pair.AccessTokenID

	require.NoError(t, svc.ChangePassword(context.Background(), uid, in.Password, "new-strong-pwd-987"))

	row, err := store.GetAccessTokenByID(context.Background(), pgutil.PgUUID(oldAccess))
	require.NoError(t, err)
	require.True(t, row.IsRevoked, "access-токен должен быть помечен is_revoked=true после смены пароля")
}
