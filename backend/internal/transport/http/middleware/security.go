package middleware

import "net/http"

// SecurityHeaders добавляет базовые security-заголовки на каждый ответ.
// X-Frame-Options защищает от clickjacking, X-Content-Type-Options
// отключает sniffing MIME, Referrer-Policy ограничивает утечку URL'ов
// при переходах. CSP здесь не задаём — у API без HTML-ответов он
// мало что даёт, для статики фронта его выставит Vite/nginx.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
