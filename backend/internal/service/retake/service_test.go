package retake_test

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
)

type fixture struct {
	store *repo.Store
	disc  *discipline.Service
	debt  *debt.Service
	svc   *retake.Service
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
	svc := retake.New(store, auditSvc, changelogSvc, nil)
	return &fixture{store: store, disc: disc, debt: debtSvc, svc: svc}
}

// seedUser создаёт пользователя и возвращает его id.
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

// seedDiscipline создаёт дисциплину и привязывает teacher+student.
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

// seedDebt ставит долг студенту от преподавателя.
func seedDebt(t *testing.T, f *fixture, student, teacher, disciplineID uuid.UUID) uuid.UUID {
	t.Helper()
	d, err := f.debt.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: disciplineID,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})
	return pgutil.UUID(d.ID)
}

// createScheduledRetake — helper: создаёт regular-пересдачу на завтра.
func createScheduledRetake(t *testing.T, f *fixture, disciplineID, creator uuid.UUID, kind string) queries.Retake {
	t.Helper()
	r, err := f.svc.Create(context.Background(), retake.CreateInput{
		DisciplineID:    disciplineID,
		Kind:            kind,
		Building:        "А",
		Room:            "101",
		ScheduledAt:     retake.NewTime(time.Now().Add(24 * time.Hour)),
		DurationMinutes: 90,
	}, creator)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM retakes WHERE id = $1", r.ID)
	})
	return r
}

func TestRetake_Create_Regular(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-create")
	student := seedUser(t, f.store, "st-create")
	discID := seedDiscipline(t, f, "RC", teacher, student)
	dean := seedUser(t, f.store, "dean-create")

	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)
	require.Equal(t, "regular", r.Kind)
	require.Equal(t, "scheduled", r.Status)
	require.Equal(t, int32(1), r.MinTeachers, "у regular min_teachers=1")
}

func TestRetake_Create_Commission_RequiresMin3(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-com")
	student := seedUser(t, f.store, "st-com")
	discID := seedDiscipline(t, f, "CM", teacher, student)
	dean := seedUser(t, f.store, "dean-com")

	r := createScheduledRetake(t, f, discID, dean, retake.KindCommission)
	require.Equal(t, "commission", r.Kind)
	require.Equal(t, int32(retake.MinCommissionTeachers), r.MinTeachers,
		"у commission min_teachers=3 (ТЗ)")
}

func TestRetake_Create_RejectsInvalidInputs(t *testing.T) {
	f := setup(t)
	dean := seedUser(t, f.store, "dean-bad")

	_, err := f.svc.Create(context.Background(), retake.CreateInput{
		DisciplineID: uuid.Nil,
		Building:     "A", Room: "1",
		ScheduledAt:     retake.NewTime(time.Now().Add(time.Hour)),
		DurationMinutes: 60,
	}, dean)
	require.ErrorIs(t, err, retake.ErrInvalidInput)

	teacher := seedUser(t, f.store, "tr-bad")
	student := seedUser(t, f.store, "st-bad")
	discID := seedDiscipline(t, f, "BAD", teacher, student)

	// Невалидный kind
	_, err = f.svc.Create(context.Background(), retake.CreateInput{
		DisciplineID: discID,
		Kind:         "unknown",
		Building:     "A", Room: "1",
		ScheduledAt:     retake.NewTime(time.Now().Add(time.Hour)),
		DurationMinutes: 60,
	}, dean)
	require.ErrorIs(t, err, retake.ErrInvalidKind)

	// Отрицательная длительность
	_, err = f.svc.Create(context.Background(), retake.CreateInput{
		DisciplineID: discID,
		Building:     "A", Room: "1",
		ScheduledAt:     retake.NewTime(time.Now().Add(time.Hour)),
		DurationMinutes: -5,
	}, dean)
	require.ErrorIs(t, err, retake.ErrInvalidInput)
}

func TestRetake_AddStudent_LinksDebt(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-as")
	student := seedUser(t, f.store, "st-as")
	dean := seedUser(t, f.store, "dean-as")
	discID := seedDiscipline(t, f, "AS", teacher, student)
	debtID := seedDebt(t, f, student, teacher, discID)
	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)

	require.NoError(t, f.svc.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	parts, err := f.svc.ListParticipants(context.Background(), pgutil.UUID(r.ID))
	require.NoError(t, err)
	require.Len(t, parts, 1)
	require.Equal(t, "student", parts[0].Kind)
	require.True(t, parts[0].DebtID.Valid)
	require.Equal(t, debtID, pgutil.UUID(parts[0].DebtID))
}

func TestRetake_AddStudent_RejectsForeignDebt(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-fd")
	studentA := seedUser(t, f.store, "stA-fd")
	studentB := seedUser(t, f.store, "stB-fd")
	dean := seedUser(t, f.store, "dean-fd")
	discID := seedDiscipline(t, f, "FD", teacher, studentA)
	// Долг studentA — пытаемся подсунуть его в участие studentB
	debtIDOfA := seedDebt(t, f, studentA, teacher, discID)
	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)

	err := f.svc.AddStudent(context.Background(), pgutil.UUID(r.ID), studentB, debtIDOfA, dean)
	require.ErrorIs(t, err, retake.ErrInvalidParticipant)
}

func TestRetake_AddTeacher_UsesCorrectKind(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-at")
	student := seedUser(t, f.store, "st-at")
	dean := seedUser(t, f.store, "dean-at")
	discID := seedDiscipline(t, f, "AT", teacher, student)

	// Regular: teacher → kind=teacher
	r1 := createScheduledRetake(t, f, discID, dean, retake.KindRegular)
	require.NoError(t, f.svc.AddTeacher(context.Background(), pgutil.UUID(r1.ID), teacher, dean))
	parts, _ := f.svc.ListParticipants(context.Background(), pgutil.UUID(r1.ID))
	require.Len(t, parts, 1)
	require.Equal(t, "teacher", parts[0].Kind)

	// Commission: teacher → kind=commission_member
	r2 := createScheduledRetake(t, f, discID, dean, retake.KindCommission)
	require.NoError(t, f.svc.AddTeacher(context.Background(), pgutil.UUID(r2.ID), teacher, dean))
	parts2, _ := f.svc.ListParticipants(context.Background(), pgutil.UUID(r2.ID))
	require.Len(t, parts2, 1)
	require.Equal(t, "commission_member", parts2[0].Kind)
}

func TestRetake_Start_RequiresEnoughTeachers(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-s")
	student := seedUser(t, f.store, "st-s")
	dean := seedUser(t, f.store, "dean-s")
	discID := seedDiscipline(t, f, "ST", teacher, student)

	// Commission — нужно 3 преподавателя
	r := createScheduledRetake(t, f, discID, dean, retake.KindCommission)
	// Без преподавателей — нельзя стартовать
	err := f.svc.Start(context.Background(), pgutil.UUID(r.ID), dean)
	require.ErrorIs(t, err, retake.ErrNotEnoughTeachers)

	// Добавляем одного — всё ещё нельзя
	require.NoError(t, f.svc.AddTeacher(context.Background(), pgutil.UUID(r.ID), teacher, dean))
	err = f.svc.Start(context.Background(), pgutil.UUID(r.ID), dean)
	require.ErrorIs(t, err, retake.ErrNotEnoughTeachers)

	// Добавляем ещё двух
	t2 := seedUser(t, f.store, "tr2-s")
	t3 := seedUser(t, f.store, "tr3-s")
	require.NoError(t, f.svc.AddTeacher(context.Background(), pgutil.UUID(r.ID), t2, dean))
	require.NoError(t, f.svc.AddTeacher(context.Background(), pgutil.UUID(r.ID), t3, dean))

	// Теперь можно стартовать
	require.NoError(t, f.svc.Start(context.Background(), pgutil.UUID(r.ID), dean))

	got, _ := f.svc.Get(context.Background(), pgutil.UUID(r.ID))
	require.Equal(t, "in_progress", got.Status)
}

func TestRetake_GradeStudent_ClosesDebtAtomically(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-g")
	student := seedUser(t, f.store, "st-g")
	dean := seedUser(t, f.store, "dean-g")
	discID := seedDiscipline(t, f, "GR", teacher, student)
	debtID := seedDebt(t, f, student, teacher, discID)
	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)

	require.NoError(t, f.svc.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))
	require.NoError(t, f.svc.AddTeacher(context.Background(), pgutil.UUID(r.ID), teacher, dean))

	require.NoError(t, f.svc.GradeStudent(context.Background(), pgutil.UUID(r.ID), student, 4, teacher))

	// 1. У участника появилась оценка
	parts, _ := f.svc.ListParticipants(context.Background(), pgutil.UUID(r.ID))
	var studentPart *queries.RetakeParticipant
	for i := range parts {
		if parts[i].Kind == "student" {
			studentPart = &parts[i]
			break
		}
	}
	require.NotNil(t, studentPart)
	require.NotNil(t, studentPart.Grade)
	require.Equal(t, int32(4), *studentPart.Grade)

	// 2. Связанный долг закрылся: status=graded, final_grade=4
	debtRow, err := f.debt.Get(context.Background(), debtID)
	require.NoError(t, err)
	require.Equal(t, "graded", debtRow.Status)
	require.NotNil(t, debtRow.FinalGrade)
	require.Equal(t, int32(4), *debtRow.FinalGrade)
}

func TestRetake_GradeStudent_RejectsRepeatedGrade(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-rg")
	student := seedUser(t, f.store, "st-rg")
	dean := seedUser(t, f.store, "dean-rg")
	discID := seedDiscipline(t, f, "RG", teacher, student)
	debtID := seedDebt(t, f, student, teacher, discID)
	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)

	require.NoError(t, f.svc.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))
	require.NoError(t, f.svc.AddTeacher(context.Background(), pgutil.UUID(r.ID), teacher, dean))
	require.NoError(t, f.svc.GradeStudent(context.Background(), pgutil.UUID(r.ID), student, 5, teacher))

	// Повторное выставление — ErrAlreadyHasGrade
	err := f.svc.GradeStudent(context.Background(), pgutil.UUID(r.ID), student, 4, teacher)
	require.ErrorIs(t, err, retake.ErrAlreadyHasGrade)
}

func TestRetake_GradeStudent_RejectsInvalidGrade(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-ig")
	student := seedUser(t, f.store, "st-ig")
	dean := seedUser(t, f.store, "dean-ig")
	discID := seedDiscipline(t, f, "IG", teacher, student)
	debtID := seedDebt(t, f, student, teacher, discID)
	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)
	require.NoError(t, f.svc.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	for _, badGrade := range []int32{0, 1, 6, 10, -3} {
		err := f.svc.GradeStudent(context.Background(), pgutil.UUID(r.ID), student, badGrade, teacher)
		require.ErrorIs(t, err, retake.ErrInvalidInput,
			"оценка %d должна быть отвергнута", badGrade)
	}
}

func TestRetake_Cancel_ReleasesRetake(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-c")
	student := seedUser(t, f.store, "st-c")
	dean := seedUser(t, f.store, "dean-c")
	discID := seedDiscipline(t, f, "CN", teacher, student)
	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)

	require.NoError(t, f.svc.Cancel(context.Background(), pgutil.UUID(r.ID), dean))

	got, _ := f.svc.Get(context.Background(), pgutil.UUID(r.ID))
	require.Equal(t, "cancelled", got.Status)

	// Повторная отмена — ErrInvalidStatus
	err := f.svc.Cancel(context.Background(), pgutil.UUID(r.ID), dean)
	require.ErrorIs(t, err, retake.ErrInvalidStatus)
}

func TestRetake_ListForUser_OnlyOwn(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-l")
	studentA := seedUser(t, f.store, "stA-l")
	studentB := seedUser(t, f.store, "stB-l")
	dean := seedUser(t, f.store, "dean-l")
	discID := seedDiscipline(t, f, "LU", teacher, studentA)
	debtA := seedDebt(t, f, studentA, teacher, discID)

	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)
	require.NoError(t, f.svc.AddStudent(context.Background(), pgutil.UUID(r.ID), studentA, debtA, dean))

	rowsA, err := f.svc.ListForUser(context.Background(), studentA)
	require.NoError(t, err)
	require.Len(t, rowsA, 1)

	rowsB, err := f.svc.ListForUser(context.Background(), studentB)
	require.NoError(t, err)
	require.Empty(t, rowsB)
}

type gradeCall struct {
	extID string
	grade emulator.Grade
	idem  string
}

// mockGradeSender потокобезопасен: write-back идёт в фоновой горутине.
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

// waitCalls ждёт n вызовов фоновой отправки и возвращает их копию.
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

func TestRetake_GradeStudent_SyncsToEmulator(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-gsync")
	student := seedUser(t, f.store, "st-gsync")
	dean := seedUser(t, f.store, "dean-gsync")
	discID := seedDiscipline(t, f, "GSYNC", teacher, student)

	sender := &mockGradeSender{}
	f.svc.SetGradeSender(sender)

	extID := "ext-retake-debt-777"
	debtRow, err := f.debt.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: discID, ExternalID: &extID,
	}, teacher)
	require.NoError(t, err)

	r := createScheduledRetake(t, f, discID, dean, retake.KindRegular)
	require.NoError(t, f.svc.AddStudent(context.Background(), pgutil.UUID(r.ID), student, pgutil.UUID(debtRow.ID), dean))
	require.NoError(t, f.svc.GradeStudent(context.Background(), pgutil.UUID(r.ID), student, 3, teacher))

	calls := sender.waitCalls(t, 1)
	require.Len(t, calls, 1, "PatchDebtGrade должен быть вызван при выставлении оценки через пересдачу")
	require.Equal(t, "ext-retake-debt-777", calls[0].extID)
	require.Equal(t, "numeric", calls[0].grade.Type)
	require.Equal(t, 3, calls[0].grade.Value)
	require.NotEmpty(t, calls[0].idem)
}
