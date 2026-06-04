package retake

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/timeutil"
)

// retakePayload — общая часть payload'а для retake-уведомлений.
// Поля сюда кладём только те, которые шаблон письма реально умеет
// отрисовать (см. notify/email.go) — discipline_name/building/room/
// scheduled_at; остальные ключи добавляют конкретные методы.
type retakePayload = map[string]any

// notifyUser шлёт одно уведомление конкретному пользователю. Ошибка только
// логируется — нотификация не должна валить успешно прошедшую мутацию
// (БД-инвариант уже соблюдён).
func (s *Service) notifyUser(ctx context.Context, userID uuid.UUID, kind string, payload retakePayload) {
	if s.notify == nil {
		return
	}
	if err := s.notify.Notify(ctx, notify.Event{
		UserID:  userID,
		Kind:    kind,
		Payload: payload,
	}); err != nil {
		slog.Warn("retake: не удалось отправить уведомление",
			"kind", kind, "user_id", userID.String(), "err", err)
	}
}

// notifyStudent оставляем как читаемую обёртку для студентских событий.
func (s *Service) notifyStudent(ctx context.Context, studentID uuid.UUID, kind string, payload retakePayload) {
	s.notifyUser(ctx, studentID, kind, payload)
}

// notifyAllStudents рассылает событие всем студентам-участникам retake.
// Используется для UpdateSchedule и Cancel. Список студентов мы тащим
// ПОСЛЕ успешной мутации, что нормально — на момент рассылки участники
// в БД уже стабильны (UPDATE retakes не трогает retake_participants).
//
// На случай гонок (студента сняли между мутацией и рассылкой) — это
// не страшно: rotated БД-данные пройдут, лишнее уведомление не отправится.
func (s *Service) notifyAllStudents(ctx context.Context, retakeID uuid.UUID, kind string, basePayload retakePayload) {
	if s.notify == nil {
		return
	}
	rows, err := s.store.ListStudentParticipantsForRetake(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		slog.Warn("retake: не удалось получить студентов для рассылки",
			"retake_id", retakeID.String(), "kind", kind, "err", err)
		return
	}
	for _, r := range rows {
		s.notifyStudent(ctx, pgutil.UUID(r.UserID), kind, basePayload)
	}
}

// notifyTeacher шлёт одно уведомление конкретному преподавателю.
// Используется для AddTeacher. Ошибка только логируется — нотификация
// не должна валить успешно прошедшую мутацию.
func (s *Service) notifyTeacher(ctx context.Context, teacherID uuid.UUID, kind string, payload retakePayload) {
	if s.notify == nil {
		return
	}
	if err := s.notify.Notify(ctx, notify.Event{
		UserID:  teacherID,
		Kind:    kind,
		Payload: payload,
	}); err != nil {
		slog.Warn("retake: не удалось отправить уведомление преподавателю",
			"kind", kind, "user_id", teacherID.String(), "err", err)
	}
}

// notifyAllTeachers рассылает событие всем преподавателям/членам комиссии
// пересдачи. Используется для UpdateSchedule и Cancel. Список тащим ПОСЛЕ
// успешной мутации — на этот момент состав участников в БД стабилен.
func (s *Service) notifyAllTeachers(ctx context.Context, retakeID uuid.UUID, kind string, basePayload retakePayload) {
	if s.notify == nil {
		return
	}
	rows, err := s.store.ListTeacherParticipantsForRetake(ctx, pgutil.PgUUID(retakeID))
	if err != nil {
		slog.Warn("retake: не удалось получить преподавателей для рассылки",
			"retake_id", retakeID.String(), "kind", kind, "err", err)
		return
	}
	for _, r := range rows {
		s.notifyTeacher(ctx, pgutil.UUID(r.UserID), kind, basePayload)
	}
}

func retakeBasePayload(retakeID uuid.UUID, disciplineID uuid.UUID, scheduledAt, building, room string) retakePayload {
	return retakePayload{
		"retake_id":     retakeID.String(),
		"discipline_id": disciplineID.String(),
		"scheduled_at":  scheduledAt,
		"building":      building,
		"room":          room,
	}
}

// retakePayloadFor строит payload с именем дисциплины и длительностью
// для email-шаблонов. Если дисциплина не найдена — поле discipline
// остаётся пустым, уведомление всё равно отправляется.
func (s *Service) retakePayloadFor(ctx context.Context, r queries.Retake) retakePayload {
	p := retakeBasePayload(
		pgutil.UUID(r.ID),
		pgutil.UUID(r.DisciplineID),
		timeutil.FormatDateTime(r.ScheduledAt.Time),
		r.Building, r.Room,
	)
	p["duration_minutes"] = r.DurationMinutes
	if disc, err := s.store.GetDisciplineByID(ctx, r.DisciplineID); err == nil {
		p["discipline"] = disc.Name
	}
	return p
}
