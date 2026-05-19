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

var emailTemplates map[string]*template.Template

func init() {
	must := func(src string) *template.Template {
		return template.Must(template.New("").Parse(src))
	}
	emailTemplates = map[string]*template.Template{
		KindRetakeScheduled: must(tmplRetakeScheduled),
		KindRetakeUpdated:   must(tmplRetakeUpdated),
		KindRetakeCancelled: must(tmplRetakeCancelled),
	}
}

var subjectByKind = map[string]string{
	KindRetakeScheduled: "Назначена пересдача",
	KindRetakeUpdated:   "Изменено расписание пересдачи",
	KindRetakeCancelled: "Пересдача отменена",
}

type emailNotifier struct {
	cfg   EmailConfig
	store *repo.Store
}

func newEmailNotifier(cfg EmailConfig, store *repo.Store) *emailNotifier {
	return &emailNotifier{cfg: cfg, store: store}
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
