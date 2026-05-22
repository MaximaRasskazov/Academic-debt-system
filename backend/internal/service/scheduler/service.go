// Package scheduler реализует автоматические переходы статусов пересдач
// по времени — это закрывает требование ТЗ:
//
//	«По истечении назначенного времени пересдача должна перейти в
//	 статус "Завершена".»
//
// Запускается отдельной горутиной из main.go и каждые `interval`
// секунд проверяет:
//   - пересдачи в статусе scheduled, у которых scheduled_at <= NOW()
//     → переводит в in_progress;
//   - пересдачи в статусе scheduled/in_progress, у которых
//     scheduled_at + duration_minutes <= NOW() → переводит в completed.
//
// Если запуск шедулера пропускается (deploy, рестарт пода) — следующий
// тик догоняет: ListRetakesToStart / ListRetakesToFinish основаны на
// фактических временных метках, а не на счётчиках.
//
// Мутации идут через Store.RunInTx с записью audit_log — это даёт
// читаемую историю "почему статус сменился" и единый паттерн с
// остальными доменными сервисами.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
)

const (
	// SystemActorID — UUID служебного user'а из 00010_seed_rbac, под
	// которым шедулер пишет audit-записи. Сам пользователь не может
	// залогиниться (password_hash пустой), но FK actor_id требует
	// валидный user.id.
	SystemActorIDStr = "00000000-0000-0000-0000-000000000000"

	actionAutoStarted  = "retake.auto_started"
	actionAutoFinished = "retake.auto_finished"

	// DefaultInterval — компромисс между нагрузкой и задержкой.
	// 1 минута даёт гарантию "пересдача завершится не позже чем
	// через минуту после end_time", при этом 60 запросов в час — это
	// ничтожная нагрузка на БД.
	DefaultInterval = time.Minute
)

// Service инкапсулирует тиковую логику.
type Service struct {
	store    *repo.Store
	audit    *audit.Service
	interval time.Duration
}

// New собирает шедулер. interval=0 → DefaultInterval.
func New(store *repo.Store, auditSvc *audit.Service, interval time.Duration) *Service {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Service{store: store, audit: auditSvc, interval: interval}
}

// Interval возвращает фактический интервал тика — нужен в тестах и
// для логирования.
func (s *Service) Interval() time.Duration { return s.interval }

// Run запускает блокирующий цикл. Останавливается по ctx.Done(),
// что позволяет main.go сделать graceful shutdown: cancel(ctx) →
// текущий tick дорабатывает → горутина выходит без хвостовых задач.
//
// Возвращает nil при штатной остановке, ошибку — если ctx закрыт с
// ошибкой (например, deadline). Внутренние ошибки тика логируются
// через slog и не валят цикл.
func (s *Service) Run(ctx context.Context) error {
	slog.Info("scheduler started", "interval", s.interval)
	// Запускаем первый тик сразу, чтобы не ждать interval при старте.
	s.Tick(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return ctx.Err()
		case <-ticker.C:
			s.Tick(ctx)
		}
	}
}

// Tick — один проход. Выделен в публичный метод, чтобы тесты могли
// дёргать его без time.Ticker.
//
// Не возвращает ошибку: внутренние сбои логируются, цикл продолжается.
// Это сознательное решение — шедулер должен быть resilient к
// временным проблемам с БД, не вылетать совсем.
func (s *Service) Tick(ctx context.Context) {
	if err := s.processStarts(ctx); err != nil {
		slog.Error("scheduler: process starts failed", "err", err)
	}
	if err := s.processFinishes(ctx); err != nil {
		slog.Error("scheduler: process finishes failed", "err", err)
	}
}

func (s *Service) processStarts(ctx context.Context) error {
	rows, err := s.store.ListRetakesToStart(ctx)
	if err != nil {
		return fmt.Errorf("list retakes to start: %w", err)
	}
	for _, r := range rows {
		if err := s.markStarted(ctx, r); err != nil {
			// Логируем и идём дальше — одна сбойная запись не должна
			// блокировать остальные.
			slog.Error("scheduler: mark started failed",
				"retake_id", r.ID, "err", err)
		}
	}
	if len(rows) > 0 {
		slog.Info("scheduler: started retakes", "count", len(rows))
	}
	return nil
}

func (s *Service) processFinishes(ctx context.Context) error {
	rows, err := s.store.ListRetakesToFinish(ctx)
	if err != nil {
		return fmt.Errorf("list retakes to finish: %w", err)
	}
	for _, r := range rows {
		if err := s.markFinished(ctx, r); err != nil {
			slog.Error("scheduler: mark finished failed",
				"retake_id", r.ID, "err", err)
		}
	}
	if len(rows) > 0 {
		slog.Info("scheduler: finished retakes", "count", len(rows))
	}
	return nil
}

// markStarted переводит scheduled → in_progress + audit в одной tx.
// MarkRetakeInProgress в SQL имеет WHERE status='scheduled' —
// если в перерыве между ListRetakesToStart и UPDATE кто-то вручную
// перевёл пересдачу (например, dean через API), мы это не сломаем.
func (s *Service) markStarted(ctx context.Context, r queries.Retake) error {
	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.MarkRetakeInProgress(ctx, r.ID); err != nil {
			return fmt.Errorf("mark in_progress: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    systemActorUUID(),
			Action:     actionAutoStarted,
			TargetType: "retake",
			TargetID:   uuidToString(r.ID),
			Details: map[string]any{
				"scheduled_at": r.ScheduledAt.Time.Format(time.RFC3339),
			},
		})
	})
}

func (s *Service) markFinished(ctx context.Context, r queries.Retake) error {
	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.MarkRetakeCompleted(ctx, r.ID); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    systemActorUUID(),
			Action:     actionAutoFinished,
			TargetType: "retake",
			TargetID:   uuidToString(r.ID),
			Details: map[string]any{
				"scheduled_at":     r.ScheduledAt.Time.Format(time.RFC3339),
				"duration_minutes": r.DurationMinutes,
			},
		})
	})
}
