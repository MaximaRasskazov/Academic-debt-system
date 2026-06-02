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
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
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
	// ErrLastStudent / ErrMinTeachers — нельзя оголить состав пересдачи:
	// должен остаться ≥1 студент и ≥min_teachers преподавателей.
	ErrLastStudent = errors.New("retake: в пересдаче должен остаться хотя бы один студент")
	ErrMinTeachers = errors.New("retake: преподавателей не может быть меньше минимально допустимого")
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
	if err := s.addParticipant(ctx, current, studentID, ParticipantStudent, &pgDebt, actorID, actionStudentAdded); err != nil {
		return err
	}

	// Уведомляем студента сразу после успешного добавления, а не при
	// создании пересдачи: до этого момента он не знает, что для него
	// что-то запланировали. retake_scheduled — главное событие в его
	// потоке нотификаций.
	s.notifyStudent(ctx, studentID, notify.KindRetakeScheduled, s.retakePayloadFor(ctx, current))
	return nil
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
	if err := s.addParticipant(ctx, current, teacherID, kind, nil, actorID, actionTeacherAdded); err != nil {
		return err
	}

	// Преподаватель узнаёт о назначении на пересдачу сразу после
	// добавления — то же событие retake_scheduled, что и у студента.
	s.notifyTeacher(ctx, teacherID, notify.KindRetakeScheduledTeacher, s.retakePayloadFor(ctx, current))
	return nil
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

	// Нельзя оголить состав: после удаления должен остаться хотя бы один
	// студент, а преподавателей — не меньше min_teachers (1 для regular,
	// ≥3 для commission). Иначе пересдачу нельзя будет провести.
	if existing.Kind == ParticipantStudent {
		students, err := s.store.ListStudentParticipantsForRetake(ctx, current.ID)
		if err != nil {
			return fmt.Errorf("count students: %w", err)
		}
		if len(students) <= 1 {
			return ErrLastStudent
		}
	} else {
		teachers, err := s.store.CountTeachersInRetake(ctx, current.ID)
		if err != nil {
			return fmt.Errorf("count teachers: %w", err)
		}
		if teachers-1 < int64(current.MinTeachers) {
			return ErrMinTeachers
		}
	}

	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
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
	if err != nil {
		return err
	}

	// Уведомляем снятого участника — симметрично Add*. Тип письма зависит
	// от роли: студенту студенческий шаблон, преподавателю/члену комиссии —
	// преподавательский. Best-effort: ошибки логируются внутри notify*.
	payload := s.retakePayloadFor(ctx, current)
	if existing.Kind == ParticipantStudent {
		s.notifyStudent(ctx, userID, notify.KindRetakeParticipantRemoved, payload)
	} else {
		s.notifyTeacher(ctx, userID, notify.KindRetakeParticipantRemovedTeacher, payload)
	}
	return nil
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

	var closedDebt queries.Debt
	if err := s.store.RunInTx(ctx, func(q *queries.Queries) error {
		// 1. Оценка участнику. SQL содержит AND grade IS NULL — при гонке
		// второй UPDATE вернёт ErrNoRows: возвращаем ErrAlreadyHasGrade,
		// а не пробрасываем raw-ошибку как 500.
		_, err := q.GradeStudentParticipant(ctx, queries.GradeStudentParticipantParams{
			ID:       participant.ID,
			Grade:    &grade,
			GradedBy: pgutil.PgUUID(gradedBy),
		})
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrAlreadyHasGrade
			}
			return fmt.Errorf("grade participant: %w", err)
		}

		// 2. Закрытие связанного долга. WHERE status='open' защищает
		// от двойного закрытия — если долг уже graded/cancelled,
		// q.GradeDebt вернёт ErrNoRows, откатит транзакцию.
		closedDebt, err = q.GradeDebt(ctx, queries.GradeDebtParams{
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
	}); err != nil {
		return err
	}

	// Обратный sync оценки в эмулятор — best-effort, в фоне.
	s.sendGradeToEmulator(closedDebt.ExternalID,
		pgutil.UUID(closedDebt.ID).String(), grade)

	// Студент должен узнать оценку сразу, не дожидаясь email-дайджеста
	// или ручного refresh. Payload минимальный — фронт сам подтянет
	// детали по retake_id, если нужно показать карточку.
	s.notifyStudent(ctx, studentID, notify.KindRetakeGradeReceived, retakePayload{
		"retake_id":     pgutil.UUID(current.ID).String(),
		"discipline_id": pgutil.UUID(current.DisciplineID).String(),
		"grade":         grade,
	})
	return nil
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
