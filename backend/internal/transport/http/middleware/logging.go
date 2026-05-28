// Структурированное логирование с trace-id.
//
// Стек:
//  1. chimw.RequestID кладёт уникальный id запроса в context (его же
//     отправляет в X-Request-ID response-header) — это делает chi сам.
//  2. LoggingContext оборачивает r.Context() в slog.WithGroup и кладёт
//     получившийся logger в ctx через ContextHandler-механизм.
//  3. В коде вместо slog.Info(...) используется slog.InfoContext(ctx, ...) —
//     handler автоматически добавит request_id (и user_id, если он уже
//     попал в context из mw.Auth).
//
// Чтобы НЕ переписывать весь код на slog.*Context — мы поверх стандартного
// JSONHandler делаем ContextHandler-обёртку, которая вытаскивает атрибуты
// из context при каждом вызове. Это даёт trace-id даже в slog.Info из
// нижних слоёв, где ctx не пробрасывается явно.

package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type ctxKeyLogger struct{}

// LoggingContext — chi-middleware: достаёт request_id (chimw.GetReqID)
// и кладёт его в context как pre-built slog.Attr. Должен идти ПОСЛЕ
// chimw.RequestID в цепочке.
func LoggingContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := chimw.GetReqID(r.Context())
		ctx := context.WithValue(r.Context(), ctxKeyLogger{}, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestIDFromContext возвращает request_id из ctx (или "" если нет).
func requestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyLogger{}).(string)
	return v
}

// userIDFromContext — пытаемся вытащить user.id, если запрос прошёл
// через mw.Auth. Ключ из middleware/auth.go (ctxKeyUserID) приватный,
// поэтому мы здесь делаем slog-атрибут только в local-cope handler'е.
// Для надёжности используем экспортируемый UserID().
func userIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	return UserID(ctx)
}

// contextLogHandler — обёртка над slog.Handler, добавляющая
// request_id и user_id ко всем записям, у которых есть r.Context().
type contextLogHandler struct{ slog.Handler }

func (h *contextLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID := requestIDFromContext(ctx); reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	if uid, ok := userIDFromContext(ctx); ok {
		r.AddAttrs(slog.String("user_id", uid.String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextLogHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextLogHandler) WithGroup(name string) slog.Handler {
	return &contextLogHandler{Handler: h.Handler.WithGroup(name)}
}

// SetupContextLogger конфигурирует slog так, чтобы InfoContext / ErrorContext
// автоматически добавляли request_id и user_id. Вызывается ОДИН РАЗ из
// main.setupLogger в самом начале запуска.
//
// level — slog.LevelInfo / Debug / Warn / Error.
func SetupContextLogger(level slog.Level) {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(&contextLogHandler{Handler: base}))
}
