package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	cleanupInterval = 5 * time.Minute
	ipIdleTimeout   = 3 * time.Minute
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiter struct {
	mu    sync.Mutex
	ips   map[string]*ipLimiter
	limit rate.Limit
	burst int
}

func newRateLimiter(limit rate.Limit, burst int) *rateLimiter {
	rl := &rateLimiter{
		ips:   make(map[string]*ipLimiter),
		limit: limit,
		burst: burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) get(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, ok := rl.ips[ip]
	if !ok {
		entry = &ipLimiter{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.ips[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, entry := range rl.ips {
			if time.Since(entry.lastSeen) > ipIdleTimeout {
				delete(rl.ips, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit возвращает middleware, ограничивающее запросы с одного IP.
// limit — скорость в событиях/секунду (например, 5.0/60 для 5/мин),
// burst — максимальный размер всплеска (обычно равен лимиту в минуту).
// При превышении отвечает 429 Too Many Requests.
func RateLimit(limit rate.Limit, burst int) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if !rl.get(ip).Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"too many requests, slow down"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
