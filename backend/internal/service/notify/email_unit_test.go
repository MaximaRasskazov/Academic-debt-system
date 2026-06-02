package notify

// Внутренние unit-тесты emailNotifier — пакет notify (не notify_test),
// чтобы иметь доступ к непубличным типам.
// БД не нужна: host="" → send() сразу возвращает nil.

import (
	"bytes"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestEmailTemplates_RenderAll проверяет, что каждый зарегистрированный
// шаблон собирается с base-layout и рендерится без ошибок на типовом
// payload. Ловит опечатки в {{define}}/{{template}} и отсутствие блоков
// "heading"/"content" — иначе они всплыли бы только в рантайме при отправке.
func TestEmailTemplates_RenderAll(t *testing.T) {
	payload := map[string]any{
		"discipline":       "Математический анализ",
		"scheduled_at":     "2026-06-10 14:30",
		"building":         "3",
		"room":             "222",
		"duration_minutes": 90,
		"grade":            4,
		"reason":           "аудитория занята",
		"decision_reason":  "согласовано с кафедрой",
		"notes":            "взять зачётку",
	}
	require.NotEmpty(t, emailTemplates)
	for kind, tmpl := range emailTemplates {
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "base", payload)
		require.NoError(t, err, "kind=%s должен рендериться", kind)
		out := buf.String()
		require.Contains(t, out, "Академический ассистент", "kind=%s: нет шапки base", kind)
		require.Contains(t, out, "<svg", "kind=%s: нет логотипа в шапке", kind)
		require.NotContains(t, out, "📋", "kind=%s: остался эмодзи в заголовке", kind)
		require.NotContains(t, out, "<no value>", "kind=%s: незаполненный плейсхолдер", kind)
		require.True(t, strings.Contains(out, "<h1>"), "kind=%s: нет заголовка", kind)
	}
}

// newTestEmailNotifier создаёт emailNotifier с пустым host (SMTP отключён)
// и без store (store.GetUserByID никогда не вызывается при host="").
func newTestEmailNotifier(queueSize int) *emailNotifier {
	e := &emailNotifier{
		cfg:   EmailConfig{}, // host="" → send() возвращает nil немедленно
		store: nil,
		queue: make(chan Event, queueSize),
		done:  make(chan struct{}),
	}
	go e.worker()
	return e
}

func TestEmailNotifier_Enqueue_NonBlocking(t *testing.T) {
	e := newTestEmailNotifier(emailQueueSize)
	defer e.close()

	ev := Event{UserID: uuid.New(), Kind: KindRetakeScheduled}

	start := time.Now()
	e.enqueue(ev)
	elapsed := time.Since(start)

	// Enqueue должен вернуться мгновенно — не ждёт SMTP.
	require.Less(t, elapsed, 50*time.Millisecond)
}

func TestEmailNotifier_Enqueue_DropsOnFullQueue(t *testing.T) {
	// Создаём нотификатор с очередью в 1 элемент и немедленно блокируем
	// воркер: для этого добавляем элемент с "занятой" очередью.
	// Проще: создаём без воркера, заполняем вручную, потом проверяем drop.

	qSize := 2
	e := &emailNotifier{
		cfg:   EmailConfig{},
		store: nil,
		queue: make(chan Event, qSize),
		done:  make(chan struct{}),
	}
	// Воркер не запущен — события в канале остаются.

	ev := Event{UserID: uuid.New(), Kind: KindRetakeScheduled}

	// Заполняем очередь до предела.
	for range qSize {
		e.enqueue(ev)
	}
	require.Len(t, e.queue, qSize, "очередь должна быть заполнена")

	// Следующий enqueue должен дропнуть (не блокироваться).
	start := time.Now()
	e.enqueue(ev) // ← дроп
	require.Less(t, time.Since(start), 500*time.Millisecond, "enqueue при полной очереди не должен блокировать")

	// Размер не изменился.
	require.Len(t, e.queue, qSize)

	// Запускаем воркер и закрываем корректно.
	go e.worker()
	e.close()
}

func TestEmailNotifier_Close_DrainsAllEnqueued(t *testing.T) {
	var processed atomic.Int32

	// Подменяем emailNotifier воркером через обычную структуру,
	// но тестируем через реальные методы enqueue/close.
	// Поскольку host="" → send() мгновенно; считаем вызовы через отдельный канал.

	// Создаём нотификатор без запуска воркера, заполняем очередь,
	// потом запускаем воркер и проверяем что close ждёт.
	const n = 10
	e := &emailNotifier{
		cfg:   EmailConfig{},
		store: nil,
		queue: make(chan Event, n),
		done:  make(chan struct{}),
	}

	ev := Event{UserID: uuid.New(), Kind: KindRetakeScheduled}
	for range n {
		e.queue <- ev
	}

	// Оборачиваем worker в подсчёт processed.
	go func() {
		defer close(e.done)
		for range e.queue {
			processed.Add(1)
		}
	}()

	e.close() // должен дождаться полного дренажа

	require.Equal(t, int32(n), processed.Load(), "все события должны быть обработаны до возврата close")
}
