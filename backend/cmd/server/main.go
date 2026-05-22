// Command server — точка входа HTTP-сервиса академических задолженностей.
//
// Старт: загружает конфиг, открывает пул соединений к Postgres, собирает
// слои (Store → TokenService → AuthService) и chi-роутер с middleware
// и маршрутами /api/auth/*. Корректно завершает работу по SIGINT/SIGTERM.
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

	"github.com/jackc/pgx/v5/pgxpool"

	_ "github.com/MaximaRasskazov/Academic-debt-system/backend/docs"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/config"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/auth"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changerequest"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/debt"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/discipline"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/rbac"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/report"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/retake"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/scheduler"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/token"
	httpx "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http"
)

const (
	dbConnectTimeout  = 10 * time.Second
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

	store := repo.NewStore(pool)
	tokens := token.New(store, []byte(cfg.JWTSecret), cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authSvc := auth.New(store, tokens)
	rbacSvc := rbac.New(store)
	auditSvc := audit.New(store)
	changelogSvc := changelog.New(store)
	disciplineSvc := discipline.New(store, auditSvc, changelogSvc)
	retakeSvc := retake.New(store, auditSvc, changelogSvc)
	reportSvc := report.New(store)

	notifyHub := notify.NewHub()
	notifySvc := notify.NewService(store, notifyHub, notify.EmailConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})

	debtSvc := debt.New(store, auditSvc, changelogSvc, disciplineSvc, notifySvc)
	changeRequestSvc := changerequest.New(store, auditSvc, changelogSvc, notifySvc)

	handler := httpx.NewRouter(httpx.Deps{
		Cfg:            cfg,
		Pool:           pool,
		Auth:           authSvc,
		Tokens:         tokens,
		RBAC:           rbacSvc,
		Disciplines:    disciplineSvc,
		Debts:          debtSvc,
		Retakes:        retakeSvc,
		ChangeRequests: changeRequestSvc,
		Reports:        reportSvc,
		Notify:         notifySvc,
		NotifyHub:      notifyHub,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	// Шедулер автопереходов retake-статусов крутится параллельно
	// HTTP-серверу. Останавливаем его через schedCancel перед
	// shutdown, чтобы не словить race на повисшем UPDATE при закрытии
	// пула соединений.
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	schedSvc := scheduler.New(store, auditSvc, scheduler.DefaultInterval)
	schedDone := make(chan struct{})
	go func() {
		defer close(schedDone)
		_ = schedSvc.Run(schedCtx)
	}()

	if err := runServer(srv, cfg); err != nil {
		return err
	}

	// Сервер уже остановлен — гасим шедулер и ждём пока он закроется,
	// прежде чем pool.Close() в defer.
	schedCancel()
	<-schedDone
	return nil
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
