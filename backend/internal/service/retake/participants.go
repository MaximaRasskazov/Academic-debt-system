package retake

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
)

// Виды участников (соответствуют CHECK в миграции 00016).
const (
	ParticipantStudent          = "student"
	ParticipantTeacher          = "teacher"
	ParticipantCommissionMember = "commission_member"

	actionStudentAdded      = "retake.student_added"
	actionStudentRemoved    = "retake.student_removed"
	actionTeacherAdded      = "retake.teacher_added"
	actionTeacherRemoved    = "retake.teacher_removed"
	actionParticipantGraded = "retake.participant_graded"
)

var (
	ErrParticipantNotFound   = errors.New("retake: участник не найден")
	ErrInvalidParticipant    = errors.New("retake: некорректные параметры участника")
	ErrAlreadyParticipant    = errors.New("retake: пользователь уже участник этой пересдачи")
	ErrStudentNeedsDebt      = errors.New("retake: студент должен быть привязан к долгу (debt_id)")
	ErrParticipantNotStudent = errors.New("retake: оценить можно только участника-студента")
)

// AddStudent добавляет студента-участника с обязательной привязкой к
// открытому долгу. БД-CHECK гарантирует kind=student ⇒ debt_id NOT NULL,
// но мы проверяем это раньше для читаемой ошибки и заодно валидируем,
// что debt действительно принадлежит этому студенту.
func (s *Service) AddStudent(ctx context.Context, retakeID, studentID, debtID, actorID uuid.UUID) error {
	if debtID == uuid.Nil {
		return ErrStudentNeedsDebt
	}
	current, err := s.Get(ctx, retakeID)
	if err != nil {
		return err
	}
	if current.Status != StatusScheduled {
		return fmt.Errorf("%w: участников можно добавлять только в scheduled-пересдачу", ErrInvalidStatus)
	}

	debtRow, err := s.store.GetDebtByID(ctx, pgutil.PgUUID(debtID))
	if err != nil {
		if repo.IsNotFound(err) {
			return fmt.Errorf("%w: debt %s не найден", ErrInvalidParticipant, debtID)
		}
		return fmt.Errorf("get debt: %w", err)
	}
	if pgutil.UUID(debtRow.StudentID) != studentID {
		return fmt.Errorf("%w: debt принадлежит другому студенту", ErrInvalidParticipant)
	}

	pgDebt := pgutil.PgUUID(debtID)
	return s.addParticipant(ctx, current, studentID, ParticipantStudent, &pgDebt, actorID, actionStudentAdded)
}

// AddTeacher добавляет преподавателя/члена комиссии. kind определяется
// видом пересдачи: regular → "teacher", commission → "commission_member".
// Различие нужно для отчётов "кто был в составе комиссии".
func (s *Service) AddTeacher(ctx context.Context, retakeID, teacherID, actorID uuid.UUID) error {
	current, err := s.Get(ctx, retakeID)
	if err != nil {
		return err
	}
	if current.Status != StatusScheduled {
		return fmt.Errorf("%w: участников можно добавлять только в scheduled-пересдачу", ErrInvalidStatus)
	}

	kind := ParticipantTeacher
	if current.Kind == KindCommission {
		kind = ParticipantCommissionMember
	}
	return s.addParticipant(ctx, current, teacherID, kind, nil, actorID, actionTeacherAdded)
}

// addParticipant — общий внутренний метод для AddStudent/AddTeacher.
// debtID для не-студентов nil, передаётся в БД как NULL.
func (s *Service) addParticipant(ctx context.Context, retake queries.Retake, userID uuid.UUID, kind string, debtID *pgtype.UUID, actorID uuid.UUID, action string) error {
	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		params := queries.AddParticipantParams{
			RetakeID: retake.ID,
			UserID:   pgutil.PgUUID(userID),
			Kind:     kind,
		}
		if debtID != nil {
			params.DebtID = *debtID
		}
		_, err := q.AddParticipant(ctx, params)
		if err != nil {
			if isUniqueViolation(err) {
				return ErrAlreadyParticipant
			}
			return fmt.Errorf("add participant: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     action,
			TargetType: entityType,
			TargetID:   pgutil.UUID(retake.ID).String(),
			Details: map[string]any{
				"user_id": userID.String(),
				"kind":    kind,
			},
		})
	})
}

// RemoveStudent — снять студента-участника. Допустимо только пока
// пересдача в scheduled и оценка ещё не выставлена.
func (s *Service) RemoveStudent(ctx context.Context, retakeID, studentID, actorID uuid.UUID) error {
	return s.removeParticipant(ctx, retakeID, studentID, ParticipantStudent, actorID, actionStudentRemoved)
}

// RemoveTeacher — снять преподавателя/члена комиссии.
func (s *Service) RemoveTeacher(ctx context.Context, retakeID, teacherID, actorID uuid.UUID) error {
	// Пустой expectedKind разрешит снять и teacher, и commission_member.
	return s.removeParticipant(ctx, retakeID, teacherID, "", actorID, actionTeacherRemoved)
}

func (s *Service) removeParticipant(ctx context.Context, retakeID, userID uuid.UUID, expectedKind string, actorID uuid.UUID, action string) error {
	current, err := s.Get(ctx, retakeID)
	if err != nil {
		return err
	}
	if current.Status != StatusScheduled {
		return fmt.Errorf("%w: участников можно убирать только из scheduled-пересдачи", ErrInvalidStatus)
	}

	existing, err := s.store.GetParticipantByRetakeAndUser(ctx, queries.GetParticipantByRetakeAndUserParams{
		RetakeID: current.ID,
		UserID:   pgutil.PgUUID(userID),
	})
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrParticipantNotFound
		}
		return fmt.Errorf("get participant: %w", err)
	}
	if expectedKind != "" && existing.Kind != expectedKind {
		return ErrParticipantNotFound
	}
	if existing.Grade != nil {
		return fmt.Errorf("%w: участнику уже выставлена оценка", ErrInvalidStatus)
	}

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.RemoveParticipant(ctx, queries.RemoveParticipantParams{
			RetakeID: current.ID,
			UserID:   pgutil.PgUUID(userID),
		}); err != nil {
			return fmt.Errorf("remove participant: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     action,
			TargetType: entityType,
			TargetID:   pgutil.UUID(current.ID).String(),
			Details:    map[string]any{"user_id": userID.String()},
		})
	})
}

// GradeStudent выставляет оценку студенту-участнику и атомарно
// закрывает связанный долг в той же транзакции.
//
// Инварианты:
//   - оценка 2..5 (БД-CHECK + явная проверка);
//   - пересдача в статусе scheduled или in_progress;
//   - участник — student с привязанным debt_id;
//   - на участнике ещё нет grade;
//   - связанный debt в статусе 'open' (иначе q.GradeDebt вернёт ErrNoRows).
//
// При невозможности любого из условий — sentinel-ошибка без побочных
// эффектов в БД (RunInTx откатит).
func (s *Service) GradeStudent(ctx context.Context, retakeID, studentID uuid.UUID, grade int32, gradedBy uuid.UUID) error {
	if grade < 2 || grade > 5 {
		return fmt.Errorf("%w: оценка должна быть 2..5", ErrInvalidInput)
	}
	current, err := s.Get(ctx, retakeID)
	if err != nil {
		return err
	}
	if current.Status != StatusScheduled && current.Status != StatusInProgress {
		return fmt.Errorf("%w: оценку можно ставить только в активной пересдаче", ErrInvalidStatus)
	}

	participant, err := s.store.GetParticipantByRetakeAndUser(ctx, queries.GetParticipantByRetakeAndUserParams{
		RetakeID: current.ID,
		UserID:   pgutil.PgUUID(studentID),
	})
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrParticipantNotFound
		}
		return fmt.Errorf("get participant: %w", err)
	}
	if participant.Kind != ParticipantStudent {
		return ErrParticipantNotStudent
	}
	if participant.Grade != nil {
		return ErrAlreadyHasGrade
	}
	if !participant.DebtID.Valid {
		return ErrStudentNeedsDebt
	}

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		// 1. Оценка участнику.
		_, err := q.GradeStudentParticipant(ctx, queries.GradeStudentParticipantParams{
			ID:       participant.ID,
			Grade:    &grade,
			GradedBy: pgutil.PgUUID(gradedBy),
		})
		if err != nil {
			return fmt.Errorf("grade participant: %w", err)
		}

		// 2. Закрытие связанного долга. WHERE status='open' защищает
		// от двойного закрытия — если долг уже graded/cancelled,
		// q.GradeDebt вернёт ErrNoRows, откатит транзакцию.
		_, err = q.GradeDebt(ctx, queries.GradeDebtParams{
			ID:         participant.DebtID,
			FinalGrade: &grade,
			GradedBy:   pgutil.PgUUID(gradedBy),
		})
		if err != nil {
			if repo.IsNotFound(err) {
				return fmt.Errorf("%w: связанный долг не в статусе open", ErrInvalidStatus)
			}
			return fmt.Errorf("grade debt: %w", err)
		}

		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    gradedBy,
			Action:     actionParticipantGraded,
			TargetType: entityType,
			TargetID:   pgutil.UUID(current.ID).String(),
			Details: map[string]any{
				"student_id":     studentID.String(),
				"participant_id": pgutil.UUID(participant.ID).String(),
				"grade":          grade,
			},
		})
	})
}

// ListParticipants возвращает всех участников пересдачи (студенты +
// преподаватели + члены комиссии). Handler-слой при желании фильтрует
// по kind.
func (s *Service) ListParticipants(ctx context.Context, retakeID uuid.UUID) ([]queries.RetakeParticipant, error) {
	rows, err := s.store.ListParticipantsForRetake(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		return nil, fmt.Errorf("list participants: %w", err)
	}
	return rows, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
