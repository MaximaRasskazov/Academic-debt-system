package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	// burst=3, limit=3/мин — первые 3 запроса должны пройти
	handler := mw.RateLimit(rate.Limit(3.0/60), 3)(okHandler())

	for i := range 3 {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "запрос %d должен пройти", i+1)
	}
}

func TestRateLimit_BlocksWhenExceeded(t *testing.T) {
	// burst=3 — четвёртый запрос должен получить 429
	handler := mw.RateLimit(rate.Limit(3.0/60), 3)(okHandler())

	for range 3 {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "192.0.2.2:5678"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "192.0.2.2:5678"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
