package discipline_test

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
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
)

func testServices(t *testing.T) (*repo.Store, *discipline.Service) {
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
	changelogSvc := changelog.New(store)
	return store, discipline.New(store, auditSvc, changelogSvc)
}

// seedUser создаёт уникального пользователя для теста.
// Возвращает uuid.UUID. Cleanup удаляет запись.
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

// uniqueDisc создаёт уникальные name/code для дисциплины, чтобы прогоны
// не конфликтовали по UNIQUE-индексам.
func uniqueDisc(prefix string) (string, string) {
	suf := time.Now().Format("150405.000000000")
	return prefix + "-" + suf, prefix + "-" + suf
}

func TestDiscipline_Create_Success(t *testing.T) {
	store, svc := testServices(t)
	actor := seedUser(t, store, "actor-create")
	name, code := uniqueDisc("MATH")

	d, err := svc.Create(context.Background(), discipline.CreateInput{
		Name: name, Code: code,
	}, actor)
	require.NoError(t, err)
	require.Equal(t, name, d.Name)
	require.Equal(t, code, d.Code)
	require.Equal(t, "manual", d.Source)

	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(),
			"DELETE FROM disciplines WHERE id = $1", d.ID)
	})

	// В audit_log должна появиться запись discipline.created.
	rows, err := store.ListRecentAudit(context.Background(), queries.ListRecentAuditParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	var foundAudit bool
	for _, r := range rows {
		if r.Action == "discipline.created" && r.TargetID != nil && *r.TargetID == pgutil.UUID(d.ID).String() {
			foundAudit = true
			break
		}
	}
	require.True(t, foundAudit, "audit_log должен содержать discipline.created")

	// В change_logs должна быть запись created с пустым before.
	logs, err := store.ListChangeLogsForEntity(context.Background(), queries.ListChangeLogsForEntityParams{
		EntityType: "discipline",
		EntityID:   pgutil.UUID(d.ID).String(),
		Limit:      10, Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	require.Equal(t, "created", logs[0].Action)
}

func TestDiscipline_Create_RejectsEmpty(t *testing.T) {
	store, svc := testServices(t)
	actor := seedUser(t, store, "actor-empty")

	_, err := svc.Create(context.Background(), discipline.CreateInput{
		Name: "  ", Code: "X",
	}, actor)
	require.ErrorIs(t, err, discipline.ErrInvalidInput)

	_, err = svc.Create(context.Background(), discipline.CreateInput{
		Name: "Math", Code: "",
	}, actor)
	require.ErrorIs(t, err, discipline.ErrInvalidInput)
}

func TestDiscipline_Create_RejectsDuplicateCode(t *testing.T) {
	store, svc := testServices(t)
	actor := seedUser(t, store, "actor-dup")
	name, code := uniqueDisc("PHYS")

	first, err := svc.Create(context.Background(), discipline.CreateInput{Name: name, Code: code}, actor)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(),
			"DELETE FROM disciplines WHERE id = $1", first.ID)
	})

	_, err = svc.Create(context.Background(), discipline.CreateInput{
		Name: name + "-other", Code: code,
	}, actor)
	require.ErrorIs(t, err, discipline.ErrCodeTaken)
}

func TestDiscipline_Update_PatchSemantics(t *testing.T) {
	store, svc := testServices(t)
	actor := seedUser(t, store, "actor-update")
	name, code := uniqueDisc("CHEM")

	d, err := svc.Create(context.Background(), discipline.CreateInput{Name: name, Code: code}, actor)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(),
			"DELETE FROM disciplines WHERE id = $1", d.ID)
	})

	newDesc := "Описание после правки"
	updated, err := svc.Update(context.Background(), pgutil.UUID(d.ID),
		discipline.UpdateInput{Description: &newDesc}, actor)
	require.NoError(t, err)
	require.NotNil(t, updated.Description)
	require.Equal(t, newDesc, *updated.Description)
	require.Equal(t, name, updated.Name, "name не должен был меняться (PATCH без поля)")
	require.Equal(t, code, updated.Code)
}

func TestDiscipline_SoftDelete_AndGetNotFound(t *testing.T) {
	store, svc := testServices(t)
	actor := seedUser(t, store, "actor-del")
	name, code := uniqueDisc("BIO")

	d, err := svc.Create(context.Background(), discipline.CreateInput{Name: name, Code: code}, actor)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(),
			"DELETE FROM disciplines WHERE id = $1", d.ID)
	})

	require.NoError(t, svc.SoftDelete(context.Background(), pgutil.UUID(d.ID), actor))

	_, err = svc.Get(context.Background(), pgutil.UUID(d.ID))
	require.ErrorIs(t, err, discipline.ErrNotFound)

	// Restore возвращает её обратно.
	require.NoError(t, svc.Restore(context.Background(), pgutil.UUID(d.ID), actor))
	got, err := svc.Get(context.Background(), pgutil.UUID(d.ID))
	require.NoError(t, err)
	require.Equal(t, d.ID, got.ID)
}

func TestDiscipline_AttachTeacher_AndList(t *testing.T) {
	store, svc := testServices(t)
	actor := seedUser(t, store, "actor-att")
	teacher := seedUser(t, store, "teacher-att")
	name, code := uniqueDisc("INF")

	d, err := svc.Create(context.Background(), discipline.CreateInput{Name: name, Code: code}, actor)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(),
			"DELETE FROM disciplines WHERE id = $1", d.ID)
	})

	require.NoError(t, svc.AttachTeacher(context.Background(), pgutil.UUID(d.ID), teacher, actor))

	teachers, err := svc.ListTeachers(context.Background(), pgutil.UUID(d.ID))
	require.NoError(t, err)
	require.Len(t, teachers, 1)
	require.Equal(t, teacher, pgutil.UUID(teachers[0].ID))

	is, err := svc.IsTeacherOf(context.Background(), teacher, pgutil.UUID(d.ID))
	require.NoError(t, err)
	require.True(t, is)

	// Повторное добавление — ErrAlreadyExists.
	err = svc.AttachTeacher(context.Background(), pgutil.UUID(d.ID), teacher, actor)
	require.True(t, errors.Is(err, discipline.ErrAlreadyExists),
		"повторная привязка должна возвращать ErrAlreadyExists, получили %v", err)

	// Detach убирает.
	require.NoError(t, svc.DetachTeacher(context.Background(), pgutil.UUID(d.ID), teacher, actor))
	is, err = svc.IsTeacherOf(context.Background(), teacher, pgutil.UUID(d.ID))
	require.NoError(t, err)
	require.False(t, is)
}

func TestDiscipline_AttachStudent_WithAcademicYear(t *testing.T) {
	store, svc := testServices(t)
	actor := seedUser(t, store, "actor-stud")
	student := seedUser(t, store, "student-att")
	name, code := uniqueDisc("HIST")

	d, err := svc.Create(context.Background(), discipline.CreateInput{Name: name, Code: code}, actor)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(),
			"DELETE FROM disciplines WHERE id = $1", d.ID)
	})

	year := "2025-2026"
	semester := int32(2)
	require.NoError(t, svc.AttachStudent(context.Background(), pgutil.UUID(d.ID), student, actor,
		discipline.AttachStudentInput{AcademicYear: &year, Semester: &semester}))

	enrolled, err := svc.IsStudentEnrolled(context.Background(), student, pgutil.UUID(d.ID))
	require.NoError(t, err)
	require.True(t, enrolled)

	list, err := svc.ListDisciplinesForStudent(context.Background(), student)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, d.ID, list[0].ID)
}

func TestDiscipline_AttachStudent_RejectsInvalidSemester(t *testing.T) {
	store, svc := testServices(t)
	actor := seedUser(t, store, "actor-bad-sem")
	student := seedUser(t, store, "student-bad-sem")
	name, code := uniqueDisc("BAD")

	d, err := svc.Create(context.Background(), discipline.CreateInput{Name: name, Code: code}, actor)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(),
			"DELETE FROM disciplines WHERE id = $1", d.ID)
	})

	badSemester := int32(3)
	err = svc.AttachStudent(context.Background(), pgutil.UUID(d.ID), student, actor,
		discipline.AttachStudentInput{Semester: &badSemester})
	require.ErrorIs(t, err, discipline.ErrInvalidInput)
}
