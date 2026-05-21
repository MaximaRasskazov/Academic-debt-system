package changerequest_test

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
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changerequest"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/debt"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retake"
)

type fixture struct {
	store  *repo.Store
	svc    *changerequest.Service
	retake *retake.Service
	disc   *discipline.Service
	debt   *debt.Service
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
	discSvc := discipline.New(store, auditSvc, changelogSvc)
	debtSvc := debt.New(store, auditSvc, changelogSvc, discSvc)
	retakeSvc := retake.New(store, auditSvc, changelogSvc)
	// notify без SMTP/WS: пустой конфиг, только DB-запись уведомлений.
	hub := notify.NewHub()
	notifySvc := notify.NewService(store, hub, notify.EmailConfig{})
	svc := changerequest.New(store, auditSvc, changelogSvc, notifySvc)

	return &fixture{
		store:  store,
		svc:    svc,
		retake: retakeSvc,
		disc:   discSvc,
		debt:   debtSvc,
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

// createScheduledRetake создаёт regular-пересдачу на завтра.
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
		_, _ = f.store.Pool().Exec(context.Background(), "DELETE FROM retakes WHERE id = $1", r.ID)
	})
	return r
}

// addTeacherParticipant добавляет преподавателя в пересдачу.
func addTeacherParticipant(t *testing.T, f *fixture, retakeID, teacherID, actorID uuid.UUID) {
	t.Helper()
	require.NoError(t, f.retake.AddTeacher(context.Background(), retakeID, teacherID, actorID))
}

// ─── тесты ──────────────────────────────────────────────────────────────────

func TestSubmit_OK(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-sub-tr")
	student := seedUser(t, f.store, "cr-sub-st")
	dean := seedUser(t, f.store, "cr-sub-dn")
	discID := seedDiscipline(t, f, "SUB", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	addTeacherParticipant(t, f, pgutil.UUID(r.ID), teacher, dean)

	newRoom := "202"
	req, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), teacher, changerequest.SubmitInput{
		Changes: changerequest.Changes{Room: &newRoom},
	})
	require.NoError(t, err)
	require.Equal(t, "pending", req.Status)
	require.Equal(t, pgutil.UUID(r.ID), req.RetakeID)
	require.NotNil(t, req.Changes.Room)
	require.Equal(t, "202", *req.Changes.Room)

	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(),
			"DELETE FROM retake_change_requests WHERE id = $1", pgutil.PgUUID(req.ID))
	})
}

func TestSubmit_RejectsNonParticipant(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-np-tr")
	student := seedUser(t, f.store, "cr-np-st")
	dean := seedUser(t, f.store, "cr-np-dn")
	outsider := seedUser(t, f.store, "cr-np-out")
	discID := seedDiscipline(t, f, "NP", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	// outsider не является участником пересдачи

	newRoom := "303"
	_, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), outsider, changerequest.SubmitInput{
		Changes: changerequest.Changes{Room: &newRoom},
	})
	require.ErrorIs(t, err, changerequest.ErrForbidden)
}

func TestSubmit_RejectsStudentParticipant(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-st-tr")
	student := seedUser(t, f.store, "cr-st-st")
	dean := seedUser(t, f.store, "cr-st-dn")
	discID := seedDiscipline(t, f, "STPART", teacher, student)
	debtID := seedDebt(t, f, student, teacher, discID)
	r := createScheduledRetake(t, f, discID, dean)
	require.NoError(t, f.retake.AddStudent(context.Background(), pgutil.UUID(r.ID), student, debtID, dean))

	newRoom := "404"
	_, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), student, changerequest.SubmitInput{
		Changes: changerequest.Changes{Room: &newRoom},
	})
	// студент — участник, но не teacher/commission_member
	require.ErrorIs(t, err, changerequest.ErrForbidden)
}

func TestSubmit_RejectsEmptyChanges(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-ec-tr")
	student := seedUser(t, f.store, "cr-ec-st")
	dean := seedUser(t, f.store, "cr-ec-dn")
	discID := seedDiscipline(t, f, "EC", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	addTeacherParticipant(t, f, pgutil.UUID(r.ID), teacher, dean)

	_, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), teacher, changerequest.SubmitInput{
		Changes: changerequest.Changes{},
	})
	require.ErrorIs(t, err, changerequest.ErrInvalidInput)
}

func TestApprove_AppliesChangesToRetake(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-ap-tr")
	student := seedUser(t, f.store, "cr-ap-st")
	dean := seedUser(t, f.store, "cr-ap-dn")
	discID := seedDiscipline(t, f, "AP", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	addTeacherParticipant(t, f, pgutil.UUID(r.ID), teacher, dean)

	newBuilding := "Б"
	newRoom := "205"
	req, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), teacher, changerequest.SubmitInput{
		Changes: changerequest.Changes{Building: &newBuilding, Room: &newRoom},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(),
			"DELETE FROM retake_change_requests WHERE id = $1", pgutil.PgUUID(req.ID))
	})

	approved, err := f.svc.Approve(context.Background(), req.ID, dean, nil)
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Status)
	require.NotNil(t, approved.ReviewedBy)
	require.Equal(t, dean, *approved.ReviewedBy)

	// Проверяем что расписание пересдачи действительно обновилось.
	updatedRetake, err := f.store.GetRetakeByID(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, "Б", updatedRetake.Building)
	require.Equal(t, "205", updatedRetake.Room)
}

func TestApprove_AlreadyProcessed(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-aa-tr")
	student := seedUser(t, f.store, "cr-aa-st")
	dean := seedUser(t, f.store, "cr-aa-dn")
	discID := seedDiscipline(t, f, "AA", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	addTeacherParticipant(t, f, pgutil.UUID(r.ID), teacher, dean)

	newRoom := "999"
	req, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), teacher, changerequest.SubmitInput{
		Changes: changerequest.Changes{Room: &newRoom},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(),
			"DELETE FROM retake_change_requests WHERE id = $1", pgutil.PgUUID(req.ID))
	})

	_, err = f.svc.Approve(context.Background(), req.ID, dean, nil)
	require.NoError(t, err)

	// Второй approve — заявка уже не pending.
	_, err = f.svc.Approve(context.Background(), req.ID, dean, nil)
	require.ErrorIs(t, err, changerequest.ErrNotPending)
}

func TestReject_OK(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-rj-tr")
	student := seedUser(t, f.store, "cr-rj-st")
	dean := seedUser(t, f.store, "cr-rj-dn")
	discID := seedDiscipline(t, f, "RJ", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	addTeacherParticipant(t, f, pgutil.UUID(r.ID), teacher, dean)

	newRoom := "111"
	req, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), teacher, changerequest.SubmitInput{
		Changes: changerequest.Changes{Room: &newRoom},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(),
			"DELETE FROM retake_change_requests WHERE id = $1", pgutil.PgUUID(req.ID))
	})

	rejected, err := f.svc.Reject(context.Background(), req.ID, dean, "аудитория занята")
	require.NoError(t, err)
	require.Equal(t, "rejected", rejected.Status)
	require.NotNil(t, rejected.DecisionReason)
	require.Equal(t, "аудитория занята", *rejected.DecisionReason)
}

func TestReject_RequiresReason(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-rr-tr")
	student := seedUser(t, f.store, "cr-rr-st")
	dean := seedUser(t, f.store, "cr-rr-dn")
	discID := seedDiscipline(t, f, "RR", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	addTeacherParticipant(t, f, pgutil.UUID(r.ID), teacher, dean)

	newRoom := "222"
	req, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), teacher, changerequest.SubmitInput{
		Changes: changerequest.Changes{Room: &newRoom},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(),
			"DELETE FROM retake_change_requests WHERE id = $1", pgutil.PgUUID(req.ID))
	})

	_, err = f.svc.Reject(context.Background(), req.ID, dean, "")
	require.ErrorIs(t, err, changerequest.ErrInvalidInput)

	_, err = f.svc.Reject(context.Background(), req.ID, dean, "   ")
	require.ErrorIs(t, err, changerequest.ErrInvalidInput)
}

func TestSubmit_RetakeNotActive(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-na-tr")
	student := seedUser(t, f.store, "cr-na-st")
	dean := seedUser(t, f.store, "cr-na-dn")
	discID := seedDiscipline(t, f, "NA", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	addTeacherParticipant(t, f, pgutil.UUID(r.ID), teacher, dean)

	// Отменяем пересдачу.
	require.NoError(t, f.retake.Cancel(context.Background(), pgutil.UUID(r.ID), dean))

	newRoom := "555"
	_, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), teacher, changerequest.SubmitInput{
		Changes: changerequest.Changes{Room: &newRoom},
	})
	require.ErrorIs(t, err, changerequest.ErrRetakeNotActive)
}

func TestReject_AlreadyProcessed(t *testing.T) {
	f := setup(t)
	teacher := seedUser(t, f.store, "cr-rap-tr")
	student := seedUser(t, f.store, "cr-rap-st")
	dean := seedUser(t, f.store, "cr-rap-dn")
	discID := seedDiscipline(t, f, "RAP", teacher, student)
	r := createScheduledRetake(t, f, discID, dean)
	addTeacherParticipant(t, f, pgutil.UUID(r.ID), teacher, dean)

	newRoom := "666"
	req, err := f.svc.Submit(context.Background(), pgutil.UUID(r.ID), teacher, changerequest.SubmitInput{
		Changes: changerequest.Changes{Room: &newRoom},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.store.Pool().Exec(context.Background(),
			"DELETE FROM retake_change_requests WHERE id = $1", pgutil.PgUUID(req.ID))
	})

	_, err = f.svc.Reject(context.Background(), req.ID, dean, "не нужно")
	require.NoError(t, err)

	// Повторный reject — уже не pending.
	_, err = f.svc.Reject(context.Background(), req.ID, dean, "снова")
	require.ErrorIs(t, err, changerequest.ErrNotPending)
}
