// Package audit реализует запись событий безопасности в audit_log.
//
// Сценарии использования:
//   - Снаружи транзакции: Log(ctx, event) — отдельная вставка через
//     основной *repo.Store.
//   - Внутри транзакции: LogTx(ctx, q, event) — вставка идёт через
//     переданный *queries.Queries, прибинденный к текущей tx, поэтому
//     запись либо коммитится вместе с основной мутацией, либо откатывается
//     при её ошибке. Это критично, иначе можно получить рассогласованный
//     audit (роль выдана, событие "role.assign" не записалось — или
//     наоборот).
//
// Action и TargetType — короткие slug'и, идущие в БД. Чтобы опечатки
// ловились компилятором, константы для часто используемых значений
// объявлены здесь и в сервис-специфичных пакетах.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// Базовые типы target'а. Сервис-специфичные target'ы (например, debt,
// retake) объявляются в самих сервисах — здесь только общие.
const (
	TargetTypeUser       = "user"
	TargetTypeRole       = "role"
	TargetTypePermission = "permission"
)

// Часто переиспользуемые action'ы. Не претендуют на полноту: каждый
// сервис добавляет свои в своём пакете (например, debt.ActionDebtCreated).
const (
	ActionUserRegistered = "user.registered"
	ActionUserLogin      = "user.login_success"
	ActionUserLogout     = "user.logout"
)

// Event — параметры одной записи audit_log.
//
// ActorID может быть uuid.Nil, если действие системное (фоновый воркер).
// TargetID — строка, потому что в БД varchar(255): обычно UUID, но
// схема позволяет не-UUID идентификаторы при необходимости.
// Details — произвольный JSON-сериализуемый payload, NULL если nil.
// IPAddress — опционально, передаётся handler'ом из request-контекста.
type Event struct {
	ActorID    uuid.UUID
	Action     string
	TargetType string
	TargetID   string
	Details    any
	IPAddress  string
}

// Service — обёртка над audit_log-запросами в Store.
type Service struct {
	store *repo.Store
}

// New собирает Service. Лёгкий, без I/O.
func New(store *repo.Store) *Service {
	return &Service{store: store}
}

// Log записывает событие отдельной транзакцией через основной пул.
// Используется когда вызывающий код не находится в транзакции
// (например, фоновый шедулер).
func (s *Service) Log(ctx context.Context, e Event) error {
	return s.write(ctx, s.store.Queries, e)
}

// LogTx записывает событие в рамках уже открытой транзакции. q —
// *queries.Queries, полученный в Store.RunInTx-callback. При ошибке
// сама транзакция откатится — audit-запись не останется без основной
// мутации.
func (s *Service) LogTx(ctx context.Context, q *queries.Queries, e Event) error {
	return s.write(ctx, q, e)
}

func (s *Service) write(ctx context.Context, q *queries.Queries, e Event) error {
	if e.Action == "" {
		return fmt.Errorf("audit: action is required")
	}
	if e.TargetType == "" {
		return fmt.Errorf("audit: target_type is required")
	}

	var details []byte
	if e.Details != nil {
		b, err := json.Marshal(e.Details)
		if err != nil {
			return fmt.Errorf("audit: marshal details: %w", err)
		}
		details = b
	}

	params := queries.CreateAuditEntryParams{
		ActorID:    pgutil.PgUUID(e.ActorID),
		Action:     e.Action,
		TargetType: e.TargetType,
		Details:    details,
	}
	if e.TargetID != "" {
		params.TargetID = &e.TargetID
	}
	if e.IPAddress != "" {
		params.IpAddress = &e.IPAddress
	}

	if _, err := q.CreateAuditEntry(ctx, params); err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}
