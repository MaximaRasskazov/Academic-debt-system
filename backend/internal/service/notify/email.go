package notify

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/google/uuid"
	"gopkg.in/gomail.v2"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
)

//go:embed templates/retake_scheduled.html
var tmplRetakeScheduled string

//go:embed templates/retake_updated.html
var tmplRetakeUpdated string

//go:embed templates/retake_cancelled.html
var tmplRetakeCancelled string

//go:embed templates/retake_grade_received.html
var tmplRetakeGradeReceived string

var emailTemplates map[string]*template.Template

func init() {
	must := func(src string) *template.Template {
		return template.Must(template.New("").Parse(src))
	}
	emailTemplates = map[string]*template.Template{
		KindRetakeScheduled:     must(tmplRetakeScheduled),
		KindRetakeUpdated:       must(tmplRetakeUpdated),
		KindRetakeCancelled:     must(tmplRetakeCancelled),
		KindRetakeGradeReceived: must(tmplRetakeGradeReceived),
	}
}

var subjectByKind = map[string]string{
	KindRetakeScheduled:     "Назначена пересдача",
	KindRetakeUpdated:       "Изменено расписание пересдачи",
	KindRetakeCancelled:     "Пересдача отменена",
	KindRetakeGradeReceived: "Получена оценка за пересдачу",
}

const emailQueueSize = 100

// emailNotifier отправляет письма через SMTP асинхронно:
// основной путь (БД + WS) не блокируется на SMTP-соединении.
// Письма ставятся в буферизованный канал и обрабатываются воркером.
// При переполнении канала событие дропается с предупреждением в лог —
// запись в БД к этому моменту уже есть, история уведомления сохранена.
type emailNotifier struct {
	cfg   EmailConfig
	store *repo.Store
	queue chan Event
	done  chan struct{}
}

func newEmailNotifier(cfg EmailConfig, store *repo.Store) *emailNotifier {
	e := &emailNotifier{
		cfg:   cfg,
		store: store,
		queue: make(chan Event, emailQueueSize),
		done:  make(chan struct{}),
	}
	go e.worker()
	return e
}

// enqueue помещает событие в очередь. Не блокирует вызывающего.
// Если очередь переполнена — логирует и дропает (best-effort канал).
func (e *emailNotifier) enqueue(ev Event) {
	select {
	case e.queue <- ev:
	default:
		slog.Warn("notify: email queue full, dropping event",
			"user_id", ev.UserID, "kind", ev.Kind)
	}
}

// close дренирует очередь и останавливает воркер. Вызывается при
// graceful shutdown сервера — гарантирует отправку писем из буфера.
func (e *emailNotifier) close() {
	close(e.queue)
	<-e.done
}

// worker читает события из канала до его закрытия.
// Ошибки SMTP не прерывают воркер — только в лог.
func (e *emailNotifier) worker() {
	defer close(e.done)
	for ev := range e.queue {
		if err := e.send(context.Background(), ev); err != nil {
			slog.Warn("notify: email worker send failed",
				"user_id", ev.UserID, "kind", ev.Kind, "err", err)
		}
	}
}

func (e *emailNotifier) send(ctx context.Context, ev Event) error {
	if e.cfg.Host == "" {
		return nil // email не настроен — пропускаем канал
	}

	tmpl, ok := emailTemplates[ev.Kind]
	if !ok {
		return nil // нет шаблона — этот тип события не требует email
	}

	userEmail, err := e.userEmail(ctx, ev.UserID)
	if err != nil {
		return fmt.Errorf("email: get user email: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, ev.Payload); err != nil {
		return fmt.Errorf("email: render %s: %w", ev.Kind, err)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", e.cfg.From)
	m.SetHeader("To", userEmail)
	m.SetHeader("Subject", subjectByKind[ev.Kind])
	m.SetBody("text/html", body.String())

	d := gomail.NewDialer(e.cfg.Host, e.cfg.Port, e.cfg.Username, e.cfg.Password)
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("email: send to %s: %w", userEmail, err)
	}

	slog.Info("email sent", "to", userEmail, "kind", ev.Kind)
	return nil
}

func (e *emailNotifier) userEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	u, err := e.store.GetUserByID(ctx, pgutil.PgUUID(userID))
	if err != nil {
		return "", err
	}
	return u.Email, nil
}
