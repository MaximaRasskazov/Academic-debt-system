package retake_test

// Интеграционные тесты для уведомлений retake (BACK-06 расширение).
// Проверяем, что после соответствующих операций в БД появляется
// запись в notifications с правильным kind и user_id студента.
// Email/WS не дёргаем — они проверяются на других уровнях, тут важна
// именно фиксация события в персистентном слое.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/debt"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retake"
)

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	u, err := uuid.Parse(s)
	require.NoError(t, err)
	return u
}

// markAllRead ставит read_at=NOW() на все уведомления пользователя.
// Используется в тестах, чтобы отделить уведомление от текущей
// операции от шумовых событий предыдущих шагов сценария.
func markAllRead(t *testing.T, store *repo.Store, userID uuid.UUID) {
	t.Helper()
	_, err := store.Pool().Exec(context.Background(),
		"UPDATE notifications SET read_at = NOW() WHERE user_id = $1 AND read_at IS NULL",
		pgutil.PgUUID(userID))
	require.NoError(t, err)
}

// fixtureWithNotify собирает retake.Service с реальным notify.Service.
// EmailConfig{} с пустым Host — email не шлётся, только запись в БД
// (см. email.go: "если cfg.Host == ” — пропускаем").
type fixtureWithNotify struct {
	*fixture
	notifySvc *notify.Service
}

func setupWithNotify(t *testing.T) *fixtureWithNotify {
	f := setup(t)
	hub := notify.NewHub()
	notifySvc := notify.NewService(f.store, hub, notify.EmailConfig{})

	auditSvc := audit.New(f.store)
	changelogSvc := changelog.New(f.store)
	disciplineSvc := discipline.New(f.store, auditSvc, changelogSvc)
	debtSvc := debt.New(f.store, auditSvc, changelogSvc, disciplineSvc, notifySvc)
	svcWithNotify := retake.New(f.store, auditSvc, changelogSvc, notifySvc)

	return &fixtureWithNotify{
		fixture:   &fixture{store: f.store, disc: f.disc, debt: debtSvc, svc: svcWithNotify},
		notifySvc: notifySvc,
	}
}

// lastNotificationKind достаёт самое свежее уведомление пользователя
// и проверяет, что оно нужного типа. Возвращает true если найдено.
//
// Используем notify.Service.ListUnread, а не raw SQL, чтобы оставаться
// в рамках публичного API сервиса (если завтра кто-то поменяет
// внутреннюю таблицу, тест не сломается).
func hasUnreadKind(t *testing.T, store *repo.Store, notifySvc *notify.Service, userID, expectedKind string) bool {
	t.Helper()
	uid := mustUUID(t, userID)
	rows, err := notifySvc.ListUnread(context.Background(), uid)
	require.NoError(t, err)
	for _, n := range rows {
		if n.Kind == expectedKind {
			return true
		}
	}
	_ = store // подавляем неиспользуемый параметр; оставлен для возможного rawSQL fallback
	return false
}

func TestNotify_RetakeScheduled_OnAddStudent(t *testing.T) {
	f := setupWithNotify(t)
	teacher := seedUser(t, f.store, "t-sched")
	student := seedUser(t, f.store, "s-sched")
	disc := seedDiscipline(t, f.fixture, "NSCH", teacher, student)
	debtID := seedDebt(t, f.fixture, student, teacher, disc)
	r := createScheduledRetake(t, f.fixture, disc, teacher, retake.KindRegular)

	require.NoError(t, f.svc.AddStudent(context.Background(),
		pgutil.UUID(r.ID), student, debtID, teacher))

	require.True(t,
		hasUnreadKind(t, f.store, f.notifySvc, student.String(), notify.KindRetakeScheduled),
		"студент должен получить retake_scheduled при добавлении в пересдачу")
}

func TestNotify_RetakeScheduled_OnAddTeacher(t *testing.T) {
	f := setupWithNotify(t)
	teacher := seedUser(t, f.store, "t-sched-teacher")
	student := seedUser(t, f.store, "s-sched-teacher")
	dean := seedUser(t, f.store, "dean-sched-teacher")
	disc := seedDiscipline(t, f.fixture, "NTSCH", teacher, student)
	r := createScheduledRetake(t, f.fixture, disc, dean, retake.KindRegular)

	require.NoError(t, f.svc.AddTeacher(context.Background(),
		pgutil.UUID(r.ID), teacher, dean))

	// Преподаватель получает отдельный тип retake_scheduled_teacher
	// (свой email-шаблон), а не студенческий retake_scheduled — см.
	// participants.go AddTeacher → notifyTeacher.
	require.True(t,
		hasUnreadKind(t, f.store, f.notifySvc, teacher.String(), notify.KindRetakeScheduledTeacher),
		"преподаватель должен получить retake_scheduled_teacher при добавлении в пересдачу")
}

func TestNotify_RetakeUpdated_OnScheduleChange(t *testing.T) {
	f := setupWithNotify(t)
	teacher := seedUser(t, f.store, "t-upd")
	student := seedUser(t, f.store, "s-upd")
	disc := seedDiscipline(t, f.fixture, "NUPD", teacher, student)
	debtID := seedDebt(t, f.fixture, student, teacher, disc)
	r := createScheduledRetake(t, f.fixture, disc, teacher, retake.KindRegular)
	require.NoError(t, f.svc.AddStudent(context.Background(),
		pgutil.UUID(r.ID), student, debtID, teacher))

	// Сбрасываем уведомления студента, чтобы исключить retake_scheduled
	// из проверки. Маркируем "прочитанным всё непрочитанное".
	markAllRead(t, f.store, student)

	newTime := retake.NewTime(time.Now().Add(48 * time.Hour))
	newRoom := "202"
	_, err := f.svc.UpdateSchedule(context.Background(),
		pgutil.UUID(r.ID),
		retake.UpdateScheduleInput{ScheduledAt: &newTime, Room: &newRoom},
		teacher)
	require.NoError(t, err)

	require.True(t,
		hasUnreadKind(t, f.store, f.notifySvc, student.String(), notify.KindRetakeUpdated),
		"студент должен получить retake_updated после изменения расписания")
}

func TestNotify_RetakeCancelled_OnCancel(t *testing.T) {
	f := setupWithNotify(t)
	teacher := seedUser(t, f.store, "t-cncl")
	student := seedUser(t, f.store, "s-cncl")
	disc := seedDiscipline(t, f.fixture, "NCNCL", teacher, student)
	debtID := seedDebt(t, f.fixture, student, teacher, disc)
	r := createScheduledRetake(t, f.fixture, disc, teacher, retake.KindRegular)
	require.NoError(t, f.svc.AddStudent(context.Background(),
		pgutil.UUID(r.ID), student, debtID, teacher))
	markAllRead(t, f.store, student)

	require.NoError(t, f.svc.Cancel(context.Background(), pgutil.UUID(r.ID), teacher))

	require.True(t,
		hasUnreadKind(t, f.store, f.notifySvc, student.String(), notify.KindRetakeCancelled),
		"студент должен получить retake_cancelled при отмене пересдачи")
}

func TestNotify_GradeReceived_OnGradeStudent(t *testing.T) {
	f := setupWithNotify(t)
	teacher := seedUser(t, f.store, "t-grd")
	student := seedUser(t, f.store, "s-grd")
	disc := seedDiscipline(t, f.fixture, "NGRD", teacher, student)
	debtID := seedDebt(t, f.fixture, student, teacher, disc)
	r := createScheduledRetake(t, f.fixture, disc, teacher, retake.KindRegular)
	require.NoError(t, f.svc.AddStudent(context.Background(),
		pgutil.UUID(r.ID), student, debtID, teacher))
	require.NoError(t, f.svc.AddTeacher(context.Background(),
		pgutil.UUID(r.ID), teacher, teacher))
	markAllRead(t, f.store, student)

	require.NoError(t, f.svc.GradeStudent(context.Background(),
		pgutil.UUID(r.ID), student, 4, teacher))

	require.True(t,
		hasUnreadKind(t, f.store, f.notifySvc, student.String(), notify.KindRetakeGradeReceived),
		"студент должен получить retake_grade_received после выставления оценки")
}

func TestNotify_NoNotifyWhenServiceIsNil(t *testing.T) {
	// Защитный тест: retake.Service с nil notify не падает на
	// операциях, требующих уведомлений. Это важно для юнит-тестов,
	// где notify-инфраструктура не нужна.
	f := setup(t) // setup создаёт svc с notify=nil
	teacher := seedUser(t, f.store, "t-nilnot")
	student := seedUser(t, f.store, "s-nilnot")
	disc := seedDiscipline(t, f, "NNIL", teacher, student)
	debtID := seedDebt(t, f, student, teacher, disc)
	r := createScheduledRetake(t, f, disc, teacher, retake.KindRegular)

	require.NoError(t, f.svc.AddStudent(context.Background(),
		pgutil.UUID(r.ID), student, debtID, teacher),
		"AddStudent должен работать и без notify.Service")
}
