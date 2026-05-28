package emulator

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestGet_RetriesOn429 — клиент должен повторить запрос после 429.
// Если на третьей попытке сервер отдаёт 200 — должен успешно распарсить.
func TestGet_RetriesOn429(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"u-1","email":"a@b","first_name":"A","last_name":"B","role":"x","status":"active"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	// Сжимаем backoff в тестах — 1с+2с подождать слишком долго для unit.
	// Backoff жёстко зашит, поэтому ждём реальные ~3 секунды.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	acc, err := c.GetAccount(ctx, "u-1")
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if acc.ID != "u-1" {
		t.Fatalf("unexpected account: %+v", acc)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

// TestGet_RateLimitedAfterMaxAttempts — если все 3 попытки получили 429,
// клиент возвращает ErrRateLimited, не маскируя ошибку.
func TestGet_RateLimitedAfterMaxAttempts(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.GetDebt(ctx, "d-1")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got: %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

// TestGet_404ReturnsNotFound — 404 не должен ретраиться,
// сразу ErrNotFound.
func TestGet_404ReturnsNotFound(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.GetDebt(ctx, "d-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("404 не должен ретраиться, попыток: %d", got)
	}
}
