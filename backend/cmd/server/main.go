// Command server — точка входа HTTP-сервиса академических задолженностей.
//
// Скелет foundation-этапа: загружает конфиг, открывает пул соединений к
// Postgres, поднимает chi-роутер с базовыми middleware и /health, корректно
// завершает работу по SIGINT/SIGTERM.
//
// Auth, RBAC, доменные модули и WebSocket будут добавлены отдельными PR.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	dbConnectTimeout  = 10 * time.Second
	dbPingTimeout     = 2 * time.Second
	requestTimeout    = 30 * time.Second
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	setupLogger()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}

	pool, err := newPool(cfg)
	if err != nil {
		return fmt.Errorf("db connect: %w", err)
	}
	defer pool.Close()
	slog.Info("database connected", "host", cfg.DBHost, "db", cfg.DBName)

	r := buildRouter(pool)

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return runServer(srv, cfg)
}

func setupLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func newPool(cfg *config.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbConnectTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}

func buildRouter(pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(requestTimeout))

	r.Get("/health", healthHandler(pool))

	return r
}

// healthHandler возвращает 200 если БД отвечает, 503 если нет.
// Используется healthcheck'ом docker-compose и k8s readiness/liveness.
func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dbPingTimeout)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_unreachable"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

// runServer запускает сервер и ждёт SIGINT/SIGTERM для graceful shutdown.
// Возвращает nil, если сервер корректно остановлен, или ошибку, если не смог.
func runServer(srv *http.Server, cfg *config.Config) error {
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", srv.Addr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-stop:
		slog.Info("shutdown signal received", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	slog.Info("server stopped cleanly")
	return nil
}
