package statement_test

// Интеграционные тесты двухэтапной ведомости против реального Postgres.
// Пропускаются, если TEST_DATABASE_URL не задан.

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
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retake"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/statement"
)

type fixture struct {
	store  *repo.Store
	disc   *discipline.Service
	debt   *debt.Service
	retake *retake.Service
	svc    *statement.Service
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
	debtSvc := debt.New(store, auditSvc, changelogSvc, disc, nil)
	retakeSvc := retake.New(store, auditSvc, changelogSvc, nil)
	svc := statement.New(store, auditSvc, nil)
	return &fixture{store: store, disc: disc, debt: debtSvc, retake: retakeSvc, svc: svc}
}

func seedUser(t *testing.T, store *repo.Store, prefix string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	email := strings.ToLower(prefix) + "+" + time.Now().Format("150405.000000000") + "@test.local"
	u, err := store.CreateUser(ctx, queries.CreateUserParams{
		Email: email, PasswordHash: "$2a$10$placeholder", FirstName: "T", LastName: "U",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.CleanupUser(ctx, store.Pool(), u.ID) })
	return pgutil.UUID(u.ID)
}

func seedDiscipline(t *testing.T, f *fixture, prefix string, teacher, student uuid.UUID) uuid.UUID {
	t.Helper()
	suf := time.Now().Format("150405.000000000")
	d, err := f.disc.Create(context.Background(), discipline.CreateInput{
		Name: prefix + "-" + suf, Code: prefix + "-" + suf,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM disciplines WHERE id = $1", d.ID)
	})
	id := pgutil.UUID(d.ID)
	require.NoError(t, f.disc.AttachTeacher(context.Background(), id, teacher, teacher))
	require.NoError(t, f.disc.AttachStudent(context.Background(), id, student, teacher, discipline.AttachStudentInput{}))
	return id
}

// seedDebtExt создаёт долг с опциональным external_id (для проверки write-back).
func seedDebtExt(t *testing.T, f *fixture, student, teacher, disciplineID uuid.UUID, extID *string) uuid.UUID {
	t.Helper()
	d, err := f.debt.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: disciplineID, ExternalID: extID,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})
	return pgutil.UUID(d.ID)
}

func createScheduledRetake(t *testing.T, f *fixture, disciplineID, creator uuid.UUID) queries.Retake {
	t.Helper()
	r, err := f.retake.Create(context.Background(), retake.CreateInput{
		DisciplineID:    disciplineID,
		Kind:            retake.KindRegular,
		Building:        "А",
		Room:            "101",
		ScheduledAt:     retake.NewTime(time.Now().Add(24 * time.Hour)),
		DurationMinutes: 90,
	}, creator)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM statement_sheets WHERE retake_id = $1", r.ID)
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM retakes WHERE id = $1", r.ID)
	})
	return r
}

// debtStatus читает текущий статус и final_grade долга.
func debtStatus(t *testing.T, f *fixture, debtID uuid.UUID) (string, *int32) {
	t.Helper()
	row, err := f.debt.Get(context.Background(), debtID)
	require.NoError(t, err)
	return row.Status, row.FinalGrade
}

// ── mock GradeSender ─────────────────────────────────────────────────────────

type gradeCall struct {
	extID string
	grade emulator.Grade
	idem  string
}

type mockGradeSender struct {
	mu    sync.Mutex
	calls []gradeCall
}

func (m *mockGradeSender) PatchDebtGrade(_ context.Context, debtExternalID string, grade emulator.Grade, _ *string, idempotencyKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, gradeCall{debtExternalID, grade, idempotencyKey})
	return nil
}

func (m *mockGradeSender) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// ── lazy-create ──────────────────────────────────────────────────────────────

func TestStatement_GetSheet_LazyCreatesOnce(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tch-lc")
	student := seedUser(t, f.store, "std-lc")
	dean := seedUser(t, f.store, "dn-lc")
	disc := seedDiscipline(t, f, "LC", teacher, student)
	r := createScheduledRetake(t, f, disc, dean)

	sheet, err := f.svc.GetSheet(context.Background(), pgutil.UUID(r.ID))
	require.NoError(t, err)
	require.Equal(t, "open", sheet.Status)

	// Повторный вызов не создаёт дубль (UNIQUE на retake_id).
	sheet2, err := f.svc.GetSheet(context.Background(), pgutil.UUID(r.ID))
	require.NoError(t, err)
	require.Equal(t, sheet.ID, sheet2.ID)
}

// ── SaveDraftGrade ───────────────────────────────────────────────────────────

func TestStatement_SaveDraftGrade_DebtStaysOpen_Overwrite(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tch-sd")
	student := seedUser(t, f.store, "std-sd")
	dean := seedUser(t, f.store, "dn-sd")
	disc := seedDiscipline(t, f, "SD", teacher, student)
	debtID := seedDebtExt(t, f, student, teacher, disc, nil)
	r := createScheduledRetake(t, f, disc, dean)
	require.NoError(t, f.retake.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	require.NoError(t, f.svc.SaveDraftGrade(context.Background(), pgutil.UUID(r.ID), student, 4, teacher))

	// Долг ОСТАЁТСЯ open — черновик не закрывает долг.
	status, grade := debtStatus(t, f, debtID)
	require.Equal(t, "open", status)
	require.Nil(t, grade)

	// Перезапись черновика разрешена (нет ErrAlreadyHasGrade).
	require.NoError(t, f.svc.SaveDraftGrade(context.Background(), pgutil.UUID(r.ID), student, 5, teacher))

	parts, _ := f.retake.ListParticipants(context.Background(), pgutil.UUID(r.ID))
	var sp *queries.RetakeParticipant
	for i := range parts {
		if parts[i].Kind == "student" {
			sp = &parts[i]
		}
	}
	require.NotNil(t, sp)
	require.NotNil(t, sp.Grade)
	require.Equal(t, int32(5), *sp.Grade)
}

func TestStatement_SaveDraftGrade_RejectsInvalid(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tch-iv")
	student := seedUser(t, f.store, "std-iv")
	dean := seedUser(t, f.store, "dn-iv")
	disc := seedDiscipline(t, f, "IV", teacher, student)
	debtID := seedDebtExt(t, f, student, teacher, disc, nil)
	r := createScheduledRetake(t, f, disc, dean)
	require.NoError(t, f.retake.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	for _, bad := range []int32{0, 1, 6, -3} {
		err := f.svc.SaveDraftGrade(context.Background(), pgutil.UUID(r.ID), student, bad, teacher)
		require.ErrorIs(t, err, statement.ErrInvalidGrade, "оценка %d должна быть отвергнута", bad)
	}
}

// ── CloseSheet ───────────────────────────────────────────────────────────────

func TestStatement_CloseSheet_GradesDebts_AndCallsEmulator(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tch-cl")
	student := seedUser(t, f.store, "std-cl")
	dean := seedUser(t, f.store, "dn-cl")
	disc := seedDiscipline(t, f, "CL", teacher, student)
	extID := "ext-debt-close-1"
	debtID := seedDebtExt(t, f, student, teacher, disc, &extID)
	r := createScheduledRetake(t, f, disc, dean)
	require.NoError(t, f.retake.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	sender := &mockGradeSender{}
	f.svc.SetGradeSender(sender)

	require.NoError(t, f.svc.SaveDraftGrade(context.Background(), pgutil.UUID(r.ID), student, 4, teacher))
	require.NoError(t, f.svc.CloseSheet(context.Background(), pgutil.UUID(r.ID), teacher))

	// Долг закрылся.
	status, grade := debtStatus(t, f, debtID)
	require.Equal(t, "graded", status)
	require.NotNil(t, grade)
	require.Equal(t, int32(4), *grade)

	// Ведомость closed.
	sheet, err := f.svc.GetSheet(context.Background(), pgutil.UUID(r.ID))
	require.NoError(t, err)
	require.Equal(t, "closed", sheet.Status)

	// Write-back в эмулятор (best-effort, фон) — дожидаемся.
	require.Eventually(t, func() bool { return sender.count() >= 1 },
		2*time.Second, 10*time.Millisecond, "ожидали write-back в эмулятор")

	// Повторное закрытие → ErrSheetClosed.
	require.ErrorIs(t, f.svc.CloseSheet(context.Background(), pgutil.UUID(r.ID), teacher), statement.ErrSheetClosed)
}

func TestStatement_CloseSheet_StudentWithoutDraft_StaysOpen(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tch-nd")
	student := seedUser(t, f.store, "std-nd")
	dean := seedUser(t, f.store, "dn-nd")
	disc := seedDiscipline(t, f, "ND", teacher, student)
	debtID := seedDebtExt(t, f, student, teacher, disc, nil)
	r := createScheduledRetake(t, f, disc, dean)
	require.NoError(t, f.retake.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	// Закрываем без проставления черновика.
	require.NoError(t, f.svc.CloseSheet(context.Background(), pgutil.UUID(r.ID), teacher))

	// Долг студента без черновика остаётся open.
	status, _ := debtStatus(t, f, debtID)
	require.Equal(t, "open", status)
}

// ── ReopenSheet ──────────────────────────────────────────────────────────────

func TestStatement_ReopenSheet_DebtsBackToOpen_DraftPreserved(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tch-rp")
	student := seedUser(t, f.store, "std-rp")
	dean := seedUser(t, f.store, "dn-rp")
	disc := seedDiscipline(t, f, "RP", teacher, student)
	debtID := seedDebtExt(t, f, student, teacher, disc, nil)
	r := createScheduledRetake(t, f, disc, dean)
	require.NoError(t, f.retake.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	require.NoError(t, f.svc.SaveDraftGrade(context.Background(), pgutil.UUID(r.ID), student, 3, teacher))
	require.NoError(t, f.svc.CloseSheet(context.Background(), pgutil.UUID(r.ID), teacher))

	// Декан открывает ведомость.
	require.NoError(t, f.svc.ReopenSheet(context.Background(), pgutil.UUID(r.ID), dean))

	// Долг вернулся в open, final_grade обнулён.
	status, grade := debtStatus(t, f, debtID)
	require.Equal(t, "open", status)
	require.Nil(t, grade)

	// Ведомость open.
	sheet, err := f.svc.GetSheet(context.Background(), pgutil.UUID(r.ID))
	require.NoError(t, err)
	require.Equal(t, "open", sheet.Status)

	// Черновик оценки на участнике СОХРАНЁН (re-open к заполненному).
	parts, _ := f.retake.ListParticipants(context.Background(), pgutil.UUID(r.ID))
	var sp *queries.RetakeParticipant
	for i := range parts {
		if parts[i].Kind == "student" {
			sp = &parts[i]
		}
	}
	require.NotNil(t, sp)
	require.NotNil(t, sp.Grade)
	require.Equal(t, int32(3), *sp.Grade)

	// Повторное открытие → ErrSheetAlreadyOpen.
	require.ErrorIs(t, f.svc.ReopenSheet(context.Background(), pgutil.UUID(r.ID), dean), statement.ErrSheetAlreadyOpen)
}

func TestStatement_ReopenSheet_DoesNotCallEmulatorReset(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tch-re")
	student := seedUser(t, f.store, "std-re")
	dean := seedUser(t, f.store, "dn-re")
	disc := seedDiscipline(t, f, "RE", teacher, student)
	extID := "ext-debt-reopen-1"
	debtID := seedDebtExt(t, f, student, teacher, disc, &extID)
	r := createScheduledRetake(t, f, disc, dean)
	require.NoError(t, f.retake.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	sender := &mockGradeSender{}
	f.svc.SetGradeSender(sender)

	require.NoError(t, f.svc.SaveDraftGrade(context.Background(), pgutil.UUID(r.ID), student, 4, teacher))
	require.NoError(t, f.svc.CloseSheet(context.Background(), pgutil.UUID(r.ID), teacher))
	require.Eventually(t, func() bool { return sender.count() >= 1 },
		2*time.Second, 10*time.Millisecond, "close должен вызвать write-back")

	before := sender.count()
	require.NoError(t, f.svc.ReopenSheet(context.Background(), pgutil.UUID(r.ID), dean))

	// Reopen НЕ шлёт сброс в эмулятор — счётчик не растёт.
	time.Sleep(200 * time.Millisecond)
	require.Equal(t, before, sender.count(), "reopen не должен звать эмулятор (нет reset-эндпоинта)")
}
