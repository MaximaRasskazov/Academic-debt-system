package debt_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/emulator"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/debt"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
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

type fixture struct {
	store *repo.Store
	disc  *discipline.Service
	svc   *debt.Service
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
	changelogSvc := changelog.New(store)
	disc := discipline.New(store, auditSvc, changelogSvc)
	svc := debt.New(store, auditSvc, changelogSvc, disc, nil)
	return &fixture{store: store, disc: disc, svc: svc}
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

// seedDiscipline создаёт дисциплину и привязывает к ней teacher + student.
// Возвращает id дисциплины.
func seedDiscipline(t *testing.T, f *fixture, prefix string, teacher, student uuid.UUID) uuid.UUID {
	t.Helper()
	suf := time.Now().Format("150405.000000000")
	d, err := f.disc.Create(context.Background(), discipline.CreateInput{
		Name: prefix + "-" + suf, Code: prefix + "-" + suf,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(),
			"DELETE FROM disciplines WHERE id = $1", d.ID)
	})
	id := pgutil.UUID(d.ID)
	require.NoError(t, f.disc.AttachTeacher(context.Background(), id, teacher, teacher))
	require.NoError(t, f.disc.AttachStudent(context.Background(), id, student, teacher, discipline.AttachStudentInput{}))
	return id
}

func TestDebt_Create_Success(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-cr")
	student := seedUser(t, f.store, "student-cr")
	discID := seedDiscipline(t, f, "CR", teacher, student)

	d, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID:    student,
		DisciplineID: discID,
	}, teacher)
	require.NoError(t, err)
	require.Equal(t, "open", d.Status)
	require.Equal(t, student, pgutil.UUID(d.StudentID))
	require.Equal(t, teacher, pgutil.UUID(d.IssuedBy))
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})

	// В audit и changelog должны появиться записи.
	logs, err := f.store.ListChangeLogsForEntity(context.Background(), queries.ListChangeLogsForEntityParams{
		EntityType: "debt", EntityID: pgutil.UUID(d.ID).String(), Limit: 5, Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	require.Equal(t, "created", logs[0].Action)
}

func TestDebt_Create_RejectsTeacherNotAssigned(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-na")
	otherTeacher := seedUser(t, f.store, "other-teacher")
	student := seedUser(t, f.store, "student-na")
	discID := seedDiscipline(t, f, "NA", teacher, student)

	// otherTeacher не ведёт эту дисциплину
	_, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID:    student,
		DisciplineID: discID,
	}, otherTeacher)
	require.ErrorIs(t, err, debt.ErrTeacherNotAssigned)
}

func TestDebt_Create_RejectsStudentNotEnrolled(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-ne")
	student := seedUser(t, f.store, "student-ne")
	otherStudent := seedUser(t, f.store, "other-student")
	discID := seedDiscipline(t, f, "NE", teacher, student)

	// otherStudent не учится на этой дисциплине
	_, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID:    otherStudent,
		DisciplineID: discID,
	}, teacher)
	require.ErrorIs(t, err, debt.ErrStudentNotEnrolled)
}

func TestDebt_Create_RejectsDuplicateOpenDebt(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-dup")
	student := seedUser(t, f.store, "student-dup")
	discID := seedDiscipline(t, f, "DUP", teacher, student)

	d, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: discID,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})

	// Вторая попытка — UNIQUE-конфликт.
	_, err = f.svc.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: discID,
	}, teacher)
	require.ErrorIs(t, err, debt.ErrDuplicateOpenDebt)
}

func TestDebt_Grade_HappyPath(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-gr")
	student := seedUser(t, f.store, "student-gr")
	discID := seedDiscipline(t, f, "GR", teacher, student)

	d, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: discID,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})

	graded, err := f.svc.Grade(context.Background(), pgutil.UUID(d.ID), 4, teacher)
	require.NoError(t, err)
	require.Equal(t, "graded", graded.Status)
	require.NotNil(t, graded.FinalGrade)
	require.Equal(t, int32(4), *graded.FinalGrade)
	require.True(t, graded.GradedAt.Valid)
	require.Equal(t, teacher, pgutil.UUID(graded.GradedBy))
}

func TestDebt_Grade_RejectsInvalidGradeAndNotOpen(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-gv")
	student := seedUser(t, f.store, "student-gv")
	discID := seedDiscipline(t, f, "GV", teacher, student)

	d, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: discID,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})

	// 1 — невалидная оценка.
	_, err = f.svc.Grade(context.Background(), pgutil.UUID(d.ID), 1, teacher)
	require.ErrorIs(t, err, debt.ErrInvalidGrade)

	// 6 — тоже невалидная.
	_, err = f.svc.Grade(context.Background(), pgutil.UUID(d.ID), 6, teacher)
	require.ErrorIs(t, err, debt.ErrInvalidGrade)

	// Закрываем долг.
	_, err = f.svc.Grade(context.Background(), pgutil.UUID(d.ID), 5, teacher)
	require.NoError(t, err)

	// Повторно поставить оценку — нельзя.
	_, err = f.svc.Grade(context.Background(), pgutil.UUID(d.ID), 4, teacher)
	require.ErrorIs(t, err, debt.ErrNotOpen)
}

func TestDebt_Cancel_HappyPath(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-cn")
	student := seedUser(t, f.store, "student-cn")
	dean := seedUser(t, f.store, "dean-cn")
	discID := seedDiscipline(t, f, "CN", teacher, student)

	d, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: discID,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})

	require.NoError(t, f.svc.Cancel(context.Background(), pgutil.UUID(d.ID), dean))

	got, err := f.svc.Get(context.Background(), pgutil.UUID(d.ID))
	require.NoError(t, err)
	require.Equal(t, "cancelled", got.Status)

	// Повторная отмена уже отменённого долга — ErrNotOpen.
	err = f.svc.Cancel(context.Background(), pgutil.UUID(d.ID), dean)
	require.ErrorIs(t, err, debt.ErrNotOpen)
}

func TestDebt_ListForStudent(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-ls")
	student := seedUser(t, f.store, "student-ls")
	discID := seedDiscipline(t, f, "LS", teacher, student)

	d, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: discID,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})

	rows, err := f.svc.ListForStudent(context.Background(), student)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, d.ID, rows[0].ID)
}

func TestDebt_ListForTeacher_OnlyOwnDisciplines(t *testing.T) {
	f := setup(t)
	teacherA := seedUser(t, f.store, "tA")
	teacherB := seedUser(t, f.store, "tB")
	student := seedUser(t, f.store, "stud-multi")

	discA := seedDiscipline(t, f, "DA", teacherA, student)
	discB := seedDiscipline(t, f, "DB", teacherB, student)

	dA, err := f.svc.Create(context.Background(), debt.CreateInput{StudentID: student, DisciplineID: discA}, teacherA)
	require.NoError(t, err)
	dB, err := f.svc.Create(context.Background(), debt.CreateInput{StudentID: student, DisciplineID: discB}, teacherB)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.store.Pool().Exec(ctx, "DELETE FROM debts WHERE id IN ($1, $2)", dA.ID, dB.ID)
	})

	// teacherA видит только долги по своим дисциплинам.
	rowsA, err := f.svc.ListForTeacher(context.Background(), teacherA, 50, 0)
	require.NoError(t, err)
	require.Len(t, rowsA, 1)
	require.Equal(t, dA.ID, rowsA[0].ID)

	// teacherB — свои.
	rowsB, err := f.svc.ListForTeacher(context.Background(), teacherB, 50, 0)
	require.NoError(t, err)
	require.Len(t, rowsB, 1)
	require.Equal(t, dB.ID, rowsB[0].ID)
}

func TestDebt_ListForTeacher_NoDisciplines_ReturnsEmpty(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-empty")

	rows, err := f.svc.ListForTeacher(context.Background(), teacher, 50, 0)
	require.NoError(t, err)
	require.Empty(t, rows)
}

type gradeCall struct {
	extID string
	grade emulator.Grade
	idem  string
}

// mockGradeSender потокобезопасен: write-back теперь идёт в отдельной
// горутине, поэтому доступ к calls защищён мьютексом, а тест ждёт
// вызова через waitCalls.
type mockGradeSender struct {
	mu    sync.Mutex
	calls []gradeCall
}

func (m *mockGradeSender) PatchDebtGrade(ctx context.Context, debtExternalID string, grade emulator.Grade, comment *string, idempotencyKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, gradeCall{debtExternalID, grade, idempotencyKey})
	return nil
}

// waitCalls ждёт, пока накопится n вызовов (фоновая горутина write-back),
// и возвращает их копию. Падает по таймауту, если вызовов меньше.
func (m *mockGradeSender) waitCalls(t *testing.T, n int) []gradeCall {
	t.Helper()
	require.Eventually(t, func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return len(m.calls) >= n
	}, 2*time.Second, 10*time.Millisecond, "ожидали %d вызовов PatchDebtGrade", n)
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]gradeCall, len(m.calls))
	copy(out, m.calls)
	return out
}

func TestDebt_Grade_SyncsToEmulator(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "teacher-sync")
	student := seedUser(t, f.store, "student-sync")
	discID := seedDiscipline(t, f, "SYNC", teacher, student)

	sender := &mockGradeSender{}
	f.svc.SetGradeSender(sender)

	extID := "ext-debt-999"
	d, err := f.svc.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: discID, ExternalID: &extID,
	}, teacher)
	require.NoError(t, err)

	_, err = f.svc.Grade(context.Background(), pgutil.UUID(d.ID), 5, teacher)
	require.NoError(t, err)

	calls := sender.waitCalls(t, 1)
	require.Len(t, calls, 1, "PatchDebtGrade должен быть вызван ровно 1 раз")
	require.Equal(t, "ext-debt-999", calls[0].extID)
	require.Equal(t, "numeric", calls[0].grade.Type)
	require.Equal(t, 5, calls[0].grade.Value)
	require.NotEmpty(t, calls[0].idem, "idempotency-ключ должен быть задан")
}
