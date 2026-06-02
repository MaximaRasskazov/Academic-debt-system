// Package statement реализует двухэтапную ведомость пересдачи.
//
// Раньше оценка по пересдаче ставилась одноэтапно и необратимо: грейд
// участнику → долг graded → write-back в эмулятор, всё в одной
// транзакции (см. retake.Service.GradeStudent в истории). Ошибку
// преподавателя откатить было нельзя.
//
// Здесь оценка расцеплена на два этапа:
//   - SaveDraftGrade — черновик на retake_participants.grade, долг
//     остаётся open, эмулятор не трогаем. Перезаписывать можно сколько
//     угодно, пока ведомость open.
//   - CloseSheet — фиксация: все долги с черновиком → graded в одной
//     транзакции + best-effort write-back в эмулятор.
//   - ReopenSheet (только декан) — откат: closed → open, долги → open,
//     черновики остаются. Эмулятор не сбрасываем (нет reset-эндпоинта).
//
// Статус ведомости (statement_sheets.status) — источник правды
// "зафиксирована ли". Запись sheet создаётся лениво при первом обращении.
package statement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/emulator"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
)

// Sentinel-ошибки. HTTP-слой маппит их в коды (см. handler.mapStatementError).
var (
	ErrSheetNotFound       = errors.New("statement: ведомость не найдена")
	ErrSheetClosed         = errors.New("statement: ведомость закрыта")    // 409
	ErrSheetAlreadyOpen    = errors.New("statement: ведомость не закрыта") // 409
	ErrRetakeNotGradeable  = errors.New("statement: пересдача не активна") // 422
	ErrInvalidGrade        = errors.New("statement: оценка должна быть 2..5")
	ErrNotStudent          = errors.New("statement: оценивать можно только студента")
	ErrParticipantNotFound = errors.New("statement: участник не найден")
	ErrStudentNeedsDebt    = errors.New("statement: у студента нет связанного долга")
	ErrRetakeNotFound      = errors.New("statement: пересдача не найдена")
)

// GradeSender инкапсулирует отправку оценки во внешнюю систему (деканат).
// Объявлен здесь, а не переиспользован из retake, чтобы не плодить
// межпакетную зависимость — контракт одинаковый, обе реализации это
// *emulator.Client. nil = обратный sync отключён.
type GradeSender interface {
	PatchDebtGrade(ctx context.Context, debtExternalID string, grade emulator.Grade, comment *string, idempotencyKey string) error
}

// Service инкапсулирует операции с ведомостью пересдачи.
//
// При закрытии ведомости связанные долги закрываются в той же
// транзакции — сервис дёргает q.GradeDebt напрямую в RunInTx-callback
// (то же сознательное нарушение strict layering ради атомарности, что
// и в retake.Service).
type Service struct {
	store       *repo.Store
	audit       *audit.Service
	notify      *notify.Service // опционально — для уведомлений студентам
	gradeSender GradeSender
}

// New собирает Service. notifySvc может быть nil (юнит-тесты).
func New(store *repo.Store, auditSvc *audit.Service, notifySvc *notify.Service) *Service {
	return &Service{store: store, audit: auditSvc, notify: notifySvc}
}

// SetGradeSender подключает внешнюю систему для write-back после init.
func (s *Service) SetGradeSender(sender GradeSender) {
	s.gradeSender = sender
}

const (
	entityType          = "statement_sheet"
	actionDraftGraded   = "statement.draft_graded"
	actionSheetClosed   = "statement.sheet_closed"
	actionSheetReopened = "statement.sheet_reopened"

	emulatorSendTimeout = 30 * time.Second
)

// ensureSheet — lazy-create ведомости. Возвращает существующую или
// только что созданную (status=open). Race-safe: CreateStatementSheet
// делает ON CONFLICT DO NOTHING, при конфликте RETURNING пуст
// (ErrNoRows) — тогда читаем уже существующую строку.
func (s *Service) ensureSheet(ctx context.Context, retakeID uuid.UUID) (queries.StatementSheet, error) {
	rid := pgutil.PgUUID(retakeID)

	created, err := s.store.CreateStatementSheet(ctx, rid)
	if err == nil {
		return created, nil
	}
	if !repo.IsNotFound(err) {
		return queries.StatementSheet{}, fmt.Errorf("create statement sheet: %w", err)
	}
	// Конфликт по UNIQUE(retake_id) — строка уже есть, читаем её.
	existing, err := s.store.GetStatementSheetByRetake(ctx, rid)
	if err != nil {
		return queries.StatementSheet{}, fmt.Errorf("get statement sheet after conflict: %w", err)
	}
	return existing, nil
}

// sendGradeToEmulator — best-effort write-back оценки в эмулятор в фоне.
// Копия логики retake.Service: своя горутина с context.Background()+
// таймаутом, ошибка только в WARN. Обратный sync вспомогательный.
func (s *Service) sendGradeToEmulator(externalID *string, debtID string, grade int32) {
	if s.gradeSender == nil || externalID == nil {
		return
	}
	extID := *externalID
	g := emulator.Grade{Type: "numeric", Value: int(grade)}
	idemKey := fmt.Sprintf("debt-grade-%s-%d", extID, grade)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), emulatorSendTimeout)
		defer cancel()
		if err := s.gradeSender.PatchDebtGrade(ctx, extID, g, nil, idemKey); err != nil {
			slog.Warn("statement: не удалось отправить оценку в эмулятор (best-effort)",
				"debt_id", debtID, "external_id", extID, "err", err)
		}
	}()
}
