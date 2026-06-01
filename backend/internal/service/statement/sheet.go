package statement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
)

const (
	participantStudent = "student"
	gradeReceivedKind  = "retake_grade_received"
)

// GetSheet — lazy-create + чтение ведомости. Валидирует, что пересдача
// существует, затем гарантирует наличие записи sheet (создаёт open,
// если её ещё нет).
func (s *Service) GetSheet(ctx context.Context, retakeID uuid.UUID) (queries.StatementSheet, error) {
	if _, err := s.store.GetRetakeByID(ctx, pgutil.PgUUID(retakeID)); err != nil {
		if repo.IsNotFound(err) {
			return queries.StatementSheet{}, ErrRetakeNotFound
		}
		return queries.StatementSheet{}, fmt.Errorf("get retake: %w", err)
	}
	return s.ensureSheet(ctx, retakeID)
}

// SaveDraftGrade пишет ЧЕРНОВИК оценки студенту. Долг НЕ закрывается,
// эмулятор НЕ трогается. Перезапись разрешена, пока ведомость open.
func (s *Service) SaveDraftGrade(ctx context.Context, retakeID, studentID uuid.UUID, grade int32, actorID uuid.UUID) error {
	if grade < 2 || grade > 5 {
		return fmt.Errorf("%w", ErrInvalidGrade)
	}

	retake, err := s.store.GetRetakeByID(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrRetakeNotFound
		}
		return fmt.Errorf("get retake: %w", err)
	}
	// Редактирование зависит от статуса ВЕДОМОСТИ, не пересдачи: препод
	// может дозаполнять оценки и после окончания слота (completed), пока
	// сам не закрыл ведомость. Отменённую пересдачу грейдить нельзя.
	if retake.Status == "cancelled" {
		return fmt.Errorf("%w", ErrRetakeNotGradeable)
	}

	sheet, err := s.ensureSheet(ctx, retakeID)
	if err != nil {
		return err
	}
	if sheet.Status == "closed" {
		return ErrSheetClosed
	}

	participant, err := s.store.GetParticipantByRetakeAndUser(ctx, queries.GetParticipantByRetakeAndUserParams{
		RetakeID: pgutil.PgUUID(retakeID),
		UserID:   pgutil.PgUUID(studentID),
	})
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrParticipantNotFound
		}
		return fmt.Errorf("get participant: %w", err)
	}
	if participant.Kind != participantStudent {
		return ErrNotStudent
	}
	if !participant.DebtID.Valid {
		return ErrStudentNeedsDebt
	}

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if _, err := q.UpsertParticipantGradeDraft(ctx, queries.UpsertParticipantGradeDraftParams{
			ID:       participant.ID,
			Grade:    &grade,
			GradedBy: pgutil.PgUUID(actorID),
		}); err != nil {
			return fmt.Errorf("upsert draft grade: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionDraftGraded,
			TargetType: entityType,
			TargetID:   pgutil.UUID(retake.ID).String(),
			Details: map[string]any{
				"student_id":     studentID.String(),
				"participant_id": pgutil.UUID(participant.ID).String(),
				"grade":          grade,
			},
		})
	})
}

// gradedDebt — план закрытия одного долга при CloseSheet.
type gradedDebt struct {
	debtID     uuid.UUID
	externalID *string
	grade      int32
	studentID  uuid.UUID
	disciID    uuid.UUID
}

// CloseSheet фиксирует ведомость: все студенты с черновиком оценки →
// долг graded, в одной транзакции; затем best-effort write-back в
// эмулятор. Студенты без черновика остаются с открытым долгом.
func (s *Service) CloseSheet(ctx context.Context, retakeID, actorID uuid.UUID) error {
	retake, err := s.store.GetRetakeByID(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrRetakeNotFound
		}
		return fmt.Errorf("get retake: %w", err)
	}

	sheet, err := s.ensureSheet(ctx, retakeID)
	if err != nil {
		return err
	}
	if sheet.Status == "closed" {
		return ErrSheetClosed
	}

	students, err := s.store.ListStudentParticipantsForRetake(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		return fmt.Errorf("list student participants: %w", err)
	}

	// Студенты с проставленным черновиком и валидным debt_id — кандидаты
	// на закрытие долга. Без черновика — долг остаётся open.
	plans := make([]gradedDebt, 0, len(students))
	for _, p := range students {
		if p.Grade == nil || !p.DebtID.Valid {
			continue
		}
		plans = append(plans, gradedDebt{
			debtID:    pgutil.UUID(p.DebtID),
			grade:     *p.Grade,
			studentID: pgutil.UUID(p.UserID),
			disciID:   pgutil.UUID(retake.DisciplineID),
		})
	}

	if err := s.store.RunInTx(ctx, func(q *queries.Queries) error {
		// Атомарно закрываем ведомость и все долги. Если хоть один долг
		// не в open (already graded/cancelled) — GradeDebt вернёт
		// ErrNoRows, и вся транзакция откатывается: частично закрытая
		// ведомость хуже, чем ни одной.
		if _, err := q.CloseStatementSheet(ctx, queries.CloseStatementSheetParams{
			RetakeID: pgutil.PgUUID(retakeID),
			ClosedBy: pgutil.PgUUID(actorID),
		}); err != nil {
			if repo.IsNotFound(err) {
				return ErrSheetClosed
			}
			return fmt.Errorf("close sheet: %w", err)
		}

		for i := range plans {
			grade := plans[i].grade
			closed, err := q.GradeDebt(ctx, queries.GradeDebtParams{
				ID:         pgutil.PgUUID(plans[i].debtID),
				FinalGrade: &grade,
				GradedBy:   pgutil.PgUUID(actorID),
			})
			if err != nil {
				if repo.IsNotFound(err) {
					return fmt.Errorf("%w: долг %s не в статусе open", ErrRetakeNotGradeable, plans[i].debtID)
				}
				return fmt.Errorf("grade debt: %w", err)
			}
			plans[i].externalID = closed.ExternalID
		}

		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionSheetClosed,
			TargetType: entityType,
			TargetID:   pgutil.UUID(retake.ID).String(),
			Details:    map[string]any{"graded_count": len(plans)},
		})
	}); err != nil {
		return err
	}

	// После коммита: write-back каждой оценки в эмулятор (best-effort) +
	// уведомление студенту о выставленной оценке.
	for i := range plans {
		s.sendGradeToEmulator(plans[i].externalID, plans[i].debtID.String(), plans[i].grade)
		s.notifyStudent(ctx, plans[i].studentID, gradeReceivedKind, map[string]any{
			"retake_id":     pgutil.UUID(retake.ID).String(),
			"discipline_id": plans[i].disciID.String(),
			"grade":         plans[i].grade,
		})
	}
	return nil
}

// ReopenSheet — ТОЛЬКО декан (роут под retakes.create). Возвращает
// закрытую ведомость в open: долги graded → open, черновики оценок на
// участниках сохраняются (re-open к заполненному черновику).
//
// Эмулятор НЕ сбрасываем: reset-эндпоинта у него нет, только
// PATCH .../grade (выставление). Отправка sentinel-значения рискует
// 422 и худшим рассинхроном. Старая оценка остаётся в эмуляторе до
// следующего CloseSheet, который перезапишет её корректным значением
// (idempotencyKey зависит от grade → новое значение уйдёт). Это
// намеренное поведение, не баг.
func (s *Service) ReopenSheet(ctx context.Context, retakeID, actorID uuid.UUID) error {
	retake, err := s.store.GetRetakeByID(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrRetakeNotFound
		}
		return fmt.Errorf("get retake: %w", err)
	}

	sheet, err := s.ensureSheet(ctx, retakeID)
	if err != nil {
		return err
	}
	if sheet.Status == "open" {
		return ErrSheetAlreadyOpen
	}

	students, err := s.store.ListStudentParticipantsForRetake(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		return fmt.Errorf("list student participants: %w", err)
	}

	var reopened, skipped int
	if err := s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if _, err := q.ReopenStatementSheet(ctx, queries.ReopenStatementSheetParams{
			RetakeID:   pgutil.PgUUID(retakeID),
			ReopenedBy: pgutil.PgUUID(actorID),
		}); err != nil {
			if repo.IsNotFound(err) {
				return ErrSheetAlreadyOpen
			}
			return fmt.Errorf("reopen sheet: %w", err)
		}

		for _, p := range students {
			if !p.DebtID.Valid {
				continue
			}
			if _, err := q.ReopenDebt(ctx, p.DebtID); err != nil {
				// 23505: sync уже создал новый open-долг по этой паре
				// (student, discipline). Пропускаем этот долг — ведомость
				// всё равно должна открыться (best-effort per debt).
				if isUniqueViolation(err) {
					skipped++
					slog.Warn("statement: reopen — долг не возвращён в open (конфликт open-долга)",
						"debt_id", pgutil.UUID(p.DebtID).String())
					continue
				}
				// ErrNoRows = долг не был graded (например без черновика) —
				// нечего откатывать, не ошибка.
				if repo.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("reopen debt: %w", err)
			}
			reopened++
		}

		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionSheetReopened,
			TargetType: entityType,
			TargetID:   pgutil.UUID(retake.ID).String(),
			Details:    map[string]any{"reopened_debts": reopened, "skipped_debts": skipped},
		})
	}); err != nil {
		return err
	}

	slog.Info("statement: ведомость открыта деканом; оценки в эмуляторе не сброшены "+
		"(нет reset-эндпоинта), будут перезаписаны при следующем закрытии",
		"retake_id", pgutil.UUID(retake.ID).String(),
		"reopened", reopened, "skipped", skipped)
	return nil
}

// notifyStudent — обёртка для студентских событий (nil-notify = no-op).
func (s *Service) notifyStudent(ctx context.Context, studentID uuid.UUID, kind string, payload map[string]any) {
	if s.notify == nil {
		return
	}
	if err := s.notify.Notify(ctx, notify.Event{UserID: studentID, Kind: kind, Payload: payload}); err != nil {
		slog.Warn("statement: не удалось отправить уведомление",
			"kind", kind, "user_id", studentID.String(), "err", err)
	}
}

// isUniqueViolation — 23505 (нарушение UNIQUE). Тот же паттерн, что в
// debt/retake сервисах.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
