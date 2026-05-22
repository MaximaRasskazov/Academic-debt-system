package report_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/debt"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/report"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retake"
)

type fixture struct {
	store  *repo.Store
	disc   *discipline.Service
	debt   *debt.Service
	retake *retake.Service
	svc    *report.Service
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
	retakeSvc := retake.New(store, auditSvc, changelogSvc)
	return &fixture{
		store: store, disc: disc, debt: debtSvc, retake: retakeSvc,
		svc: report.New(store),
	}
}

func seedUser(t *testing.T, store *repo.Store, prefix string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	email := strings.ToLower(prefix) + "+" + time.Now().Format("150405.000000000") + "@test.local"
	u, err := store.CreateUser(ctx, queries.CreateUserParams{
		Email:        email,
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "Иван",
		LastName:     "Тестов",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.Pool().Exec(ctx, "DELETE FROM users WHERE id = $1", u.ID)
	})
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

func TestReport_DebtsSummary_AggregatesByDiscipline(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-sum")
	student := seedUser(t, f.store, "st-sum")
	disc := seedDiscipline(t, f, "RP", teacher, student)

	// Создаём долг
	d, err := f.debt.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: disc,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})

	rows, err := f.svc.DebtsSummary(context.Background())
	require.NoError(t, err)

	// В сводке должна быть наша дисциплина с open=1, graded=0.
	var found bool
	for _, r := range rows {
		if r.DisciplineID == disc {
			require.Equal(t, int64(1), r.OpenCount, "ожидаем 1 открытый долг")
			require.Equal(t, int64(0), r.GradedCount)
			require.NotEmpty(t, r.DisciplineName, "имя дисциплины должно подтянуться")
			require.NotEmpty(t, r.DisciplineCode, "код дисциплины должен подтянуться")
			found = true
			break
		}
	}
	require.True(t, found, "наша дисциплина должна быть в сводке")
}

func TestReport_RetakesForPeriod_RejectsInvalidPeriod(t *testing.T) {
	f := setup(t)
	now := time.Now()
	_, err := f.svc.RetakesForPeriod(context.Background(), now, now.Add(-time.Hour))
	require.ErrorIs(t, err, report.ErrInvalidPeriod)

	_, err = f.svc.RetakesForPeriod(context.Background(), now, now)
	require.ErrorIs(t, err, report.ErrInvalidPeriod)
}

func TestReport_RetakesForPeriod_EmptyResult(t *testing.T) {
	f := setup(t)
	// Очень узкий диапазон в прошлом — гарантированно нет пересдач.
	from := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(1980, 1, 2, 0, 0, 0, 0, time.UTC)

	rows, err := f.svc.RetakesForPeriod(context.Background(), from, to)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestReport_RetakesForPeriod_IncludesCompletedWithParticipants(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "tr-c")
	student := seedUser(t, f.store, "st-c")
	dean := seedUser(t, f.store, "dean-c")
	disc := seedDiscipline(t, f, "REP", teacher, student)

	// Долг → пересдача → grade → completed
	d, err := f.debt.Create(context.Background(), debt.CreateInput{
		StudentID: student, DisciplineID: disc,
	}, teacher)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM debts WHERE id = $1", d.ID)
	})

	r, err := f.retake.Create(context.Background(), retake.CreateInput{
		DisciplineID:    disc,
		Kind:            retake.KindRegular,
		Building:        "Б",
		Room:            "201",
		ScheduledAt:     retake.NewTime(time.Now().Add(-2 * time.Hour)),
		DurationMinutes: 60,
	}, dean)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM retakes WHERE id = $1", r.ID)
	})

	require.NoError(t, f.retake.AddStudent(context.Background(), pgutil.UUID(r.ID), student, pgutil.UUID(d.ID), dean))
	require.NoError(t, f.retake.AddTeacher(context.Background(), pgutil.UUID(r.ID), teacher, dean))
	require.NoError(t, f.retake.GradeStudent(context.Background(), pgutil.UUID(r.ID), student, 4, teacher))
	require.NoError(t, f.retake.Complete(context.Background(), pgutil.UUID(r.ID), dean))

	// Запрашиваем отчёт за окно вокруг completed_at.
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(24 * time.Hour)

	rows, err := f.svc.RetakesForPeriod(context.Background(), from, to)
	require.NoError(t, err)

	var ourRow *report.RetakeReportRow
	for i := range rows {
		if rows[i].RetakeID == pgutil.UUID(r.ID) {
			ourRow = &rows[i]
			break
		}
	}
	require.NotNil(t, ourRow, "наша пересдача должна быть в отчёте")
	require.Equal(t, "Б", ourRow.Building)
	require.Equal(t, "201", ourRow.Room)
	require.Equal(t, int32(60), ourRow.DurationMinutes)
	require.Len(t, ourRow.Students, 1)
	require.NotNil(t, ourRow.Students[0].Grade)
	require.Equal(t, int32(4), *ourRow.Students[0].Grade)
	require.NotEmpty(t, ourRow.Students[0].FullName)
	require.NotEmpty(t, ourRow.Students[0].Email)
	require.Len(t, ourRow.Teachers, 1)
}

func TestReport_ExportCSV_HasBOMAndUTF8(t *testing.T) {
	rows := []report.RetakeReportRow{
		{
			DisciplineName:  "Математика",
			DisciplineCode:  "MATH-101",
			Kind:            "regular",
			Building:        "А",
			Room:            "101",
			CompletedAt:     time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC),
			DurationMinutes: 90,
			Students: []report.ParticipantInfo{
				{FullName: "Иванов Иван", Email: "ivan@test.local", Grade: int32Ptr(5)},
			},
			Teachers: []report.ParticipantInfo{
				{FullName: "Петров Пётр"},
			},
		},
	}

	body, err := report.ExportRetakes(rows, report.FormatCSV)
	require.NoError(t, err)
	require.NotEmpty(t, body)

	// UTF-8 BOM в начале (для Excel под Windows).
	require.True(t, bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}), "ожидается UTF-8 BOM")

	text := string(body)
	require.Contains(t, text, "Математика")
	require.Contains(t, text, "Иванов Иван")
	require.Contains(t, text, "Петров Пётр")
	require.Contains(t, text, "5")
}

func TestReport_ExportXLSX_CanBeOpened(t *testing.T) {
	rows := []report.RetakeReportRow{
		{
			DisciplineName:  "Физика",
			DisciplineCode:  "PHYS-201",
			Kind:            "commission",
			Building:        "Б",
			Room:            "302",
			CompletedAt:     time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
			DurationMinutes: 120,
			Students: []report.ParticipantInfo{
				{FullName: "Сидоров Алексей", Grade: int32Ptr(4)},
			},
		},
	}

	body, err := report.ExportRetakes(rows, report.FormatXLSX)
	require.NoError(t, err)
	require.NotEmpty(t, body)

	// Парсим как Excel — должно открыться без ошибок.
	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	sheetRows, err := f.GetRows("Пересдачи")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sheetRows), 2, "должна быть строка заголовка + хотя бы одна строка данных")

	// Первая строка — заголовки.
	require.Equal(t, "Дата проведения", sheetRows[0][0])
	require.Equal(t, "Дисциплина (код)", sheetRows[0][1])

	// Вторая строка — наши данные.
	require.Equal(t, "PHYS-201", sheetRows[1][1])
	require.Equal(t, "Физика", sheetRows[1][2])
	require.Equal(t, "Сидоров Алексей", sheetRows[1][7])
	require.Equal(t, "4", sheetRows[1][10])
}

func TestReport_ExportRetakes_RejectsUnknownFormat(t *testing.T) {
	_, err := report.ExportRetakes(nil, report.ExportFormat("pdf"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported format")
}

func int32Ptr(v int32) *int32 { return &v }
