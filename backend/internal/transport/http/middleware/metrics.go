// Prometheus-метрики HTTP-слоя.
//
// Экспонируется три метрики:
//   - http_requests_total{method, route, code}      — счётчик запросов
//   - http_request_duration_seconds{method, route}  — histogram длительности
//   - http_requests_in_flight                       — gauge активных запросов
//
// Endpoint: GET /metrics (без auth — внутри сети контейнеров. В проде
// прячется через basic-auth nginx или сетевыми политиками).
//
// route-label берётся из chi-роутинга (RoutePattern), не из реального
// URL — это критично для cardinality: иначе каждый UUID в /api/debts/:id
// создал бы отдельный временной ряд, что бы взорвало Prometheus.

package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Общее количество HTTP-запросов",
		},
		[]string{"method", "route", "code"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Длительность HTTP-запроса в секундах",
			Buckets: prometheus.DefBuckets, // 5ms..10s — для backend ОК
		},
		[]string{"method", "route"},
	)

	httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Количество HTTP-запросов в обработке прямо сейчас",
		},
	)
)

// nolint:gochecknoinits // регистрация метрик при импорте — стандартный паттерн client_golang
func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpRequestsInFlight)
}

// statusRecorder перехватывает StatusCode из http.ResponseWriter,
// чтобы записать его в Prometheus после ServeHTTP.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// RequestMetrics — chi-middleware: считает запросы, измеряет их
// длительность и держит gauge in-flight. Должен идти В САМОМ НИЗУ
// цепочки middleware, чтобы измерять всю обработку запроса включая
// auth/CORS/rate-limit.
func RequestMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		// 200 — дефолт, который применит net/http если handler не вызвал
		// WriteHeader явно (например, просто w.Write дал 200 неявно).
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(rec, r)

		// chi.RouteContext доступен только ПОСЛЕ matching'а — поэтому
		// мы читаем его здесь, после ServeHTTP. RoutePattern даёт шаблон
		// вроде "/api/debts/{id}", не реальный путь с UUID.
		route := "unmatched"
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			if p := rctx.RoutePattern(); p != "" {
				route = p
			}
		}

		httpRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// MetricsHandler возвращает http.Handler для эндпоинта /metrics в
// формате Prometheus. Регистрируется в роутере БЕЗ auth-middleware:
// в production-инфре /metrics закрывается nginx или network policy,
// внутри docker-compose доступен только из docker-сети.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
