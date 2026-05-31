package emulator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// withFastBackoff подменяет глобальные RetryAfter/MaxAttempts на
// тестовые значения и восстанавливает их в Cleanup. Это позволяет
// проверять retry-логику без 35×N секунд ожидания.
func withFastBackoff(t *testing.T, attempts int) {
	t.Helper()
	prevRetry, prevMax := RetryAfter, MaxAttempts
	RetryAfter = 10 * time.Millisecond
	MaxAttempts = attempts
	t.Cleanup(func() {
		RetryAfter = prevRetry
		MaxAttempts = prevMax
	})
}

// TestGet_RetriesOn429 — клиент должен повторить запрос после 429.
// Если на третьей попытке сервер отдаёт 200 — должен успешно распарсить.
func TestGet_RetriesOn429(t *testing.T) {
	withFastBackoff(t, 3)
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

// TestGet_RateLimitedAfterMaxAttempts — если все попытки получили 429,
// клиент возвращает ErrRateLimited, не маскируя ошибку.
func TestGet_RateLimitedAfterMaxAttempts(t *testing.T) {
	withFastBackoff(t, 3)
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

func TestPatchDebtGrade_Success(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/debts/ext-123/grade" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Grade          Grade   `json:"grade"`
			Comment        *string `json:"comment"`
			IdempotencyKey string  `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode error: %v", err)
		}
		if body.Grade.Type != "numeric" {
			t.Errorf("expected grade.type numeric, got %q", body.Grade.Type)
		}
		if int(body.Grade.Value.(float64)) != 4 {
			t.Errorf("expected grade.value 4, got %v", body.Grade.Value)
		}
		if body.IdempotencyKey == "" {
			t.Error("expected non-empty idempotency_key")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	err := c.PatchDebtGrade(context.Background(), "ext-123",
		Grade{Type: "numeric", Value: 4}, nil, "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestPatchDebtGrade_Rejected422(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"code":"validation_error","details":{"field":"grade.type","reason":"допустимы numeric и pass_fail"}}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	err := c.PatchDebtGrade(context.Background(), "ext-123",
		Grade{Type: "numeric", Value: 4}, nil, "key-1")
	if !errors.Is(err, ErrGradeRejected) {
		t.Fatalf("expected ErrGradeRejected, got: %v", err)
	}
}

func TestPatchDebtGrade_EmulatorError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	err := c.PatchDebtGrade(context.Background(), "ext-123",
		Grade{Type: "numeric", Value: 4}, nil, "key-1")
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
	if errors.Is(err, ErrGradeRejected) {
		t.Fatal("500 must not map to ErrGradeRejected")
	}
}
