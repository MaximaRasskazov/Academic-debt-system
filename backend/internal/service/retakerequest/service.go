// Package retakerequest реализует заявки преподавателей на СОЗДАНИЕ
// новой пересдачи (отличается от changerequest — там про изменение
// существующей).
//
// Жизненный цикл: pending → approved | rejected.
//
// Approve в одной транзакции:
//  1. Помечает заявку approved;
//  2. Создаёт пересдачу (q.CreateRetake);
//  3. Прикрепляет студентов и преподавателей через q.AddParticipant;
//  4. Записывает created_retake_id обратно в заявку;
//  5. Пишет changelog + audit_log.
//
// Это сознательное прямое использование q.* в обход retake.Service —
// тот же паттерн, что в changerequest.Approve (см. doc там). Альтернатива
// (transactional functor поверх двух сервисов) не оправдывает сложности
// для одной операции.
package retakerequest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/notify"
)

const (
	entityType = "retake_request"

	actionSubmitted = "retake_request.submitted"
	actionApproved  = "retake_request.approved"
	actionRejected  = "retake_request.rejected"

	// Те же константы, что в retake.Service — дублирую, чтобы не
	// импортировать пакет retake ради двух строк (избегаем цикла,
	// если retake захочет когда-то вызвать retakerequest).
	kindRegular    = "regular"
	kindCommission = "commission"

	participantStudent          = "student"
	participantTeacher          = "teacher"
	participantCommissionMember = "commission_member"

	minRegularTeachers    int32 = 1
	minCommissionTeachers int32 = 3
)

// Sentinel-ошибки. HTTP-слой маппит их в 400/403/404/409/422.
var (
	ErrNotFound     = errors.New("retakerequest: заявка не найдена")
	ErrNotPending   = errors.New("retakerequest: заявка уже обработана")
	ErrInvalidInput = errors.New("retakerequest: некорректные параметры")
	ErrInvalidKind  = errors.New("retakerequest: kind должен быть regular или commission")
	// ErrScheduledInPast — время пересдачи уже прошло к моменту одобрения.
	// Создавать такую пересдачу нельзя: шедулер сразу переведёт её в
	// in_progress («Идёт»), хотя ожидается «Назначена». Декан должен
	// отклонить заявку или попросить преподавателя подать новую.
	ErrScheduledInPast = errors.New("retakerequest: время пересдачи уже прошло")
)

// Payload — содержимое заявки. Маршалится в JSONB при сохранении.
//
// student_debt_ids и teacher_ids — uuid'ы; debt-id, потому что для
// прикрепления студента к пересдаче нужен конкретный долг
// (см. retake.Service.AddStudent).
type Payload struct {
	DisciplineID    uuid.UUID   `json:"discipline_id"`
	Kind            string      `json:"kind"` // regular / commission
	ScheduledAt     time.Time   `json:"scheduled_at"`
	DurationMinutes int32       `json:"duration_minutes"`
	Building        string      `json:"building"`
	Room            string      `json:"room"`
	Notes           *string     `json:"notes,omitempty"`
	Reason          *string     `json:"reason,omitempty"`
	StudentDebtIDs  []uuid.UUID `json:"student_debt_ids,omitempty"`
	TeacherIDs      []uuid.UUID `json:"teacher_ids,omitempty"`
}

// Request — доменная модель. Наружу sqlc-структуры не текут.
type Request struct {
	ID              uuid.UUID
	RequestedBy     uuid.UUID
	Payload         Payload
	Status          string
	ReviewedBy      *uuid.UUID
	ReviewedAt      *time.Time
	DecisionReason  *string
	CreatedRetakeID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

// Service инкапсулирует операции с заявками на создание пересдачи.
type Service struct {
	store     *repo.Store
	audit     *audit.Service
	changelog *changelog.Service
	notify    *notify.Service // опционально, для уведомлений
}

func New(store *repo.Store, a *audit.Service, cl *changelog.Service, n *notify.Service) *Service {
	return &Service{store: store, audit: a, changelog: cl, notify: n}
}

// Submit подаёт заявку от имени actorID.
// Permission retakes.request проверяется в HTTP-слое router.
func (s *Service) Submit(ctx context.Context, actorID uuid.UUID, p Payload) (Request, error) {
	if err := validatePayload(&p); err != nil {
		return Request{}, err
	}

	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return Request{}, fmt.Errorf("marshal payload: %w", err)
	}

	var created queries.RetakeRequest
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.CreateRetakeRequest(ctx, queries.CreateRetakeRequestParams{
			RequestedBy: pgutil.PgUUID(actorID),
			Payload:     payloadJSON,
		})
		if err != nil {
			return fmt.Errorf("create retake request: %w", err)
		}
		created = row
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionSubmitted,
			TargetType: entityType,
			TargetID:   pgutil.UUID(row.ID).String(),
			Details:    map[string]any{"discipline_id": p.DisciplineID.String(), "kind": p.Kind},
		})
	})
	if err != nil {
		return Request{}, err
	}
	return fromRow(created)
}

// ListPending — деканат: входящие заявки.
func (s *Service) ListPending(ctx context.Context, limit, offset int32) ([]Request, error) {
	rows, err := s.store.ListPendingRetakeRequests(ctx, queries.ListPendingRetakeRequestsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	return fromRows(rows)
}

// ListMy — преподаватель: его собственные заявки.
func (s *Service) ListMy(ctx context.Context, teacherID uuid.UUID, limit, offset int32) ([]Request, error) {
	rows, err := s.store.ListRetakeRequestsForTeacher(ctx, queries.ListRetakeRequestsForTeacherParams{
		RequestedBy: pgutil.PgUUID(teacherID),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list my: %w", err)
	}
	return fromRows(rows)
}

// Get возвращает заявку по id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Request, error) {
	row, err := s.store.GetRetakeRequestByID(ctx, pgutil.PgUUID(id))
	if err != nil {
		if repo.IsNotFound(err) {
			return Request{}, ErrNotFound
		}
		return Request{}, fmt.Errorf("get retake request: %w", err)
	}
	return fromRow(row)
}

// Approve одобряет заявку: атомарно создаёт пересдачу и прикрепляет
// участников. После коммита — best-effort уведомления.
func (s *Service) Approve(ctx context.Context, id, actorID uuid.UUID, decisionReason *string) (Request, error) {
	req, err := s.Get(ctx, id)
	if err != nil {
		return Request{}, err
	}
	if req.Status != "pending" {
		return Request{}, ErrNotPending
	}
	p := req.Payload

	// minTeachers по типу пересдачи. Логика дублирует retake.Service.Create —
	// дублирование сознательное, см. doc пакета.
	var minTeachers int32
	switch p.Kind {
	case kindRegular:
		minTeachers = minRegularTeachers
	case kindCommission:
		minTeachers = minCommissionTeachers
	default:
		return Request{}, ErrInvalidKind
	}

	// Время пересдачи должно быть в будущем на момент одобрения: иначе
	// созданная пересдача мгновенно уедет в in_progress по шедулеру.
	if !p.ScheduledAt.After(time.Now()) {
		return Request{}, ErrScheduledInPast
	}

	var approved queries.RetakeRequest
	var createdRetake queries.Retake
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		// 1) Создаём пересдачу.
		retake, err := q.CreateRetake(ctx, queries.CreateRetakeParams{
			DisciplineID:    pgutil.PgUUID(p.DisciplineID),
			Kind:            p.Kind,
			MinTeachers:     minTeachers,
			Building:        strings.TrimSpace(p.Building),
			Room:            strings.TrimSpace(p.Room),
			ScheduledAt:     pgtype.Timestamptz{Time: p.ScheduledAt, Valid: true},
			DurationMinutes: p.DurationMinutes,
			CreatedBy:       pgutil.PgUUID(actorID),
			Notes:           p.Notes,
		})
		if err != nil {
			return fmt.Errorf("create retake: %w", err)
		}
		createdRetake = retake

		// 2) Прикрепляем студентов (kind='student' + debt_id).
		for _, debtID := range p.StudentDebtIDs {
			debt, err := q.GetDebtByID(ctx, pgutil.PgUUID(debtID))
			if err != nil {
				return fmt.Errorf("get debt %s: %w", debtID, err)
			}
			pgDebtID := pgutil.PgUUID(debtID)
			if _, err := q.AddParticipant(ctx, queries.AddParticipantParams{
				RetakeID: retake.ID,
				UserID:   debt.StudentID,
				Kind:     participantStudent,
				DebtID:   pgDebtID,
			}); err != nil {
				return fmt.Errorf("add student participant: %w", err)
			}
		}

		// 3) Прикрепляем преподавателей. Для commission — kind='commission_member',
		// для regular — 'teacher'. На уровне БД это просто строка в CHECK,
		// но семантика разная — см. ListTeacherParticipantsForRetake.
		teacherKind := participantTeacher
		if p.Kind == kindCommission {
			teacherKind = participantCommissionMember
		}
		for _, tID := range p.TeacherIDs {
			if _, err := q.AddParticipant(ctx, queries.AddParticipantParams{
				RetakeID: retake.ID,
				UserID:   pgutil.PgUUID(tID),
				Kind:     teacherKind,
				DebtID:   pgtype.UUID{},
			}); err != nil {
				return fmt.Errorf("add teacher participant: %w", err)
			}
		}

		// 4) Помечаем заявку approved + записываем created_retake_id.
		row, err := q.ApproveRetakeRequest(ctx, queries.ApproveRetakeRequestParams{
			ID:              pgutil.PgUUID(id),
			ReviewedBy:      pgutil.PgUUID(actorID),
			DecisionReason:  decisionReason,
			CreatedRetakeID: retake.ID,
		})
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrNotPending
			}
			return fmt.Errorf("approve: %w", err)
		}
		approved = row

		// 5) changelog по созданной пересдаче + audit_log по заявке.
		if err := s.changelog.LogCreatedTx(ctx, q, "retake", pgutil.UUID(retake.ID).String(),
			retakeFields(retake), actorID); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionApproved,
			TargetType: entityType,
			TargetID:   pgutil.UUID(approved.ID).String(),
			Details: map[string]any{
				"retake_id":     pgutil.UUID(retake.ID).String(),
				"discipline_id": p.DisciplineID.String(),
			},
		})
	})
	if err != nil {
		return Request{}, err
	}

	// Уведомляем подавшего, что заявка одобрена и пересдача создана.
	if s.notify != nil {
		if err := s.notify.Notify(ctx, notify.Event{
			UserID: req.RequestedBy,
			Kind:   notify.KindRetakeChangeApproved, // переиспользуем близкое по смыслу событие
			Payload: map[string]any{
				"retake_request_id": pgutil.UUID(approved.ID).String(),
				"retake_id":         pgutil.UUID(createdRetake.ID).String(),
			},
		}); err != nil {
			slog.Warn("retakerequest: уведомление не отправлено", "err", err)
		}
	}

	return fromRow(approved)
}

// Reject отклоняет заявку. decisionReason обязателен.
func (s *Service) Reject(ctx context.Context, id, actorID uuid.UUID, decisionReason string) (Request, error) {
	if strings.TrimSpace(decisionReason) == "" {
		return Request{}, fmt.Errorf("%w: причина отклонения обязательна", ErrInvalidInput)
	}

	req, err := s.Get(ctx, id)
	if err != nil {
		return Request{}, err
	}
	if req.Status != "pending" {
		return Request{}, ErrNotPending
	}

	var rejected queries.RetakeRequest
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.RejectRetakeRequest(ctx, queries.RejectRetakeRequestParams{
			ID:             pgutil.PgUUID(id),
			ReviewedBy:     pgutil.PgUUID(actorID),
			DecisionReason: decisionReason,
		})
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrNotPending
			}
			return fmt.Errorf("reject: %w", err)
		}
		rejected = row
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionRejected,
			TargetType: entityType,
			TargetID:   pgutil.UUID(rejected.ID).String(),
			Details:    map[string]any{"decision_reason": decisionReason},
		})
	})
	if err != nil {
		return Request{}, err
	}

	if s.notify != nil {
		_ = s.notify.Notify(ctx, notify.Event{
			UserID: req.RequestedBy,
			Kind:   notify.KindRetakeChangeRejected,
			Payload: map[string]any{
				"retake_request_id": pgutil.UUID(rejected.ID).String(),
				"decision_reason":   decisionReason,
			},
		})
	}
	return fromRow(rejected)
}

// validatePayload проверяет обязательные поля и нормализует kind.
func validatePayload(p *Payload) error {
	if p.DisciplineID == uuid.Nil {
		return fmt.Errorf("%w: discipline_id обязателен", ErrInvalidInput)
	}
	if strings.TrimSpace(p.Building) == "" || strings.TrimSpace(p.Room) == "" {
		return fmt.Errorf("%w: building и room обязательны", ErrInvalidInput)
	}
	if p.ScheduledAt.IsZero() {
		return fmt.Errorf("%w: scheduled_at обязателен", ErrInvalidInput)
	}
	if p.DurationMinutes <= 0 {
		return fmt.Errorf("%w: duration_minutes должна быть положительной", ErrInvalidInput)
	}
	if p.Kind == "" {
		p.Kind = kindRegular
	}
	if p.Kind != kindRegular && p.Kind != kindCommission {
		return ErrInvalidKind
	}
	// Для commission в момент approve мы будем создавать пересдачу
	// с min_teachers=3 — БД-CHECK retakes_commission_min_teachers иначе
	// откажет. Проверим число teacher_ids уже здесь чтобы преподаватель
	// получил понятную ошибку на submit, а не позже на approve.
	// gosec G115: len() возвращает int, который на 32-битных платформах
	// формально может превышать int32. Сравниваем напрямую int с int —
	// никакого переполнения.
	if p.Kind == kindCommission && len(p.TeacherIDs) < int(minCommissionTeachers) {
		return fmt.Errorf("%w: для комиссии нужно минимум 3 преподавателя", ErrInvalidInput)
	}
	return nil
}

// retakeFields формирует срез полей retake для change_logs.
// Дублирует приватный retake.retakeFields — не экспортируем, чтобы не
// плодить публичную поверхность пакета retake ради одного места.
func retakeFields(r queries.Retake) changelog.Fields {
	f := changelog.Fields{
		"discipline_id":    pgutil.UUID(r.DisciplineID).String(),
		"kind":             r.Kind,
		"building":         r.Building,
		"room":             r.Room,
		"scheduled_at":     r.ScheduledAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		"duration_minutes": r.DurationMinutes,
		"status":           r.Status,
	}
	if r.Notes != nil {
		f["notes"] = *r.Notes
	}
	return f
}

// fromRow конвертирует sqlc-структуру в доменную модель.
func fromRow(r queries.RetakeRequest) (Request, error) {
	var payload Payload
	if err := json.Unmarshal(r.Payload, &payload); err != nil {
		return Request{}, fmt.Errorf("retakerequest: unmarshal payload: %w", err)
	}
	out := Request{
		ID:             pgutil.UUID(r.ID),
		RequestedBy:    pgutil.UUID(r.RequestedBy),
		Payload:        payload,
		Status:         r.Status,
		DecisionReason: r.DecisionReason,
		CreatedAt:      r.CreatedAt.Time,
	}
	if r.ReviewedBy.Valid {
		id := pgutil.UUID(r.ReviewedBy)
		out.ReviewedBy = &id
	}
	if r.ReviewedAt.Valid {
		t := r.ReviewedAt.Time
		out.ReviewedAt = &t
	}
	if r.CreatedRetakeID.Valid {
		id := pgutil.UUID(r.CreatedRetakeID)
		out.CreatedRetakeID = &id
	}
	if r.UpdatedAt.Valid {
		t := r.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out, nil
}

func fromRows(rows []queries.RetakeRequest) ([]Request, error) {
	out := make([]Request, 0, len(rows))
	for _, r := range rows {
		req, err := fromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, nil
}
