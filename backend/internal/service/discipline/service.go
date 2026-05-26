// Package discipline реализует управление справочником дисциплин и
// привязками студентов/преподавателей к ним.
//
// Состав сервиса:
//   - CRUD дисциплин (Create / Get / List / Update / SoftDelete / Restore);
//   - привязка преподавателей (AttachTeacher / DetachTeacher / ListTeachers /
//     ListDisciplinesForTeacher / IsTeacherOf);
//   - привязка студентов (AttachStudent / DetachStudent / ListStudents /
//     ListDisciplinesForStudent / IsStudentEnrolled).
//
// Все мутации идут через Store.RunInTx с записью audit_log + change_logs
// в той же транзакции — атомарная мутация + история.
package discipline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/audit"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/changelog"
)

// Семантические константы.
const (
	entityType   = "discipline"
	sourceManual = "manual"

	actionCreated         = "discipline.created"
	actionUpdated         = "discipline.updated"
	actionSoftDeleted     = "discipline.soft_deleted"
	actionRestored        = "discipline.restored"
	actionTeacherAttached = "discipline.teacher_attached"
	actionTeacherDetached = "discipline.teacher_detached"
	actionStudentAttached = "discipline.student_attached"
	actionStudentDetached = "discipline.student_detached"
)

// Sentinel-ошибки. HTTP-слой маппит их в 400/404/409.
var (
	ErrNotFound        = errors.New("discipline: не найдена")
	ErrCodeTaken       = errors.New("discipline: code уже занят")
	ErrNameTaken       = errors.New("discipline: name уже занят")
	ErrExternalIDTaken = errors.New("discipline: external_id уже используется")
	ErrInvalidInput    = errors.New("discipline: некорректные параметры")
	ErrAlreadyExists   = errors.New("discipline: привязка уже существует")
)

// Service инкапсулирует операции с дисциплинами.
type Service struct {
	store     *repo.Store
	audit     *audit.Service
	changelog *changelog.Service
}

func New(store *repo.Store, auditSvc *audit.Service, changelogSvc *changelog.Service) *Service {
	return &Service{store: store, audit: auditSvc, changelog: changelogSvc}
}

// CreateInput — параметры создания. ExternalID/Source опциональны
// (заполняются при синхронизации с внешней системой).
type CreateInput struct {
	Name        string
	Code        string
	Description *string
	ExternalID  *string
	Source      string // 'manual' (по умолчанию) или 'sync'
}

// UpdateInput — PATCH-семантика: NULL не меняет поле.
type UpdateInput struct {
	Name        *string
	Code        *string
	Description *string
}

// Create создаёт дисциплину и пишет аудит+changelog в одной tx.
func (s *Service) Create(ctx context.Context, in CreateInput, actorID uuid.UUID) (queries.Discipline, error) {
	name := strings.TrimSpace(in.Name)
	code := strings.TrimSpace(in.Code)
	if name == "" || code == "" {
		return queries.Discipline{}, fmt.Errorf("%w: name и code обязательны", ErrInvalidInput)
	}
	if len([]rune(name)) > 255 {
		return queries.Discipline{}, fmt.Errorf("%w: name не может превышать 255 символов", ErrInvalidInput)
	}
	if len([]rune(code)) > 50 {
		return queries.Discipline{}, fmt.Errorf("%w: code не может превышать 50 символов", ErrInvalidInput)
	}
	source := in.Source
	if source == "" {
		source = sourceManual
	}
	if source != "manual" && source != "sync" {
		return queries.Discipline{}, fmt.Errorf("%w: source должен быть manual или sync", ErrInvalidInput)
	}

	// Проверяем уникальность code и name до INSERT — UNIQUE-индекс ловит
	// и сам, но явная проверка даёт читаемую ошибку для API.
	if _, err := s.store.GetDisciplineByCode(ctx, code); err == nil {
		return queries.Discipline{}, ErrCodeTaken
	} else if !repo.IsNotFound(err) {
		return queries.Discipline{}, fmt.Errorf("check code: %w", err)
	}

	var created queries.Discipline
	err := s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.CreateDiscipline(ctx, queries.CreateDisciplineParams{
			Name:        name,
			Code:        code,
			Description: in.Description,
			ExternalID:  in.ExternalID,
			Source:      source,
			CreatedBy:   pgutil.PgUUID(actorID),
		})
		if err != nil {
			if repo.IsUniqueViolation(err, "idx_disciplines_name_lower") {
				return ErrNameTaken
			}
			if repo.IsUniqueViolation(err, "idx_disciplines_external_id") {
				return ErrExternalIDTaken
			}
			return fmt.Errorf("create: %w", err)
		}
		created = row

		if err := s.changelog.LogCreatedTx(ctx, q, entityType, pgutil.UUID(row.ID).String(),
			disciplineFields(row), actorID); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionCreated,
			TargetType: entityType,
			TargetID:   pgutil.UUID(row.ID).String(),
			Details:    map[string]any{"name": row.Name, "code": row.Code},
		})
	})
	if err != nil {
		return queries.Discipline{}, err
	}
	return created, nil
}

// Get возвращает дисциплину по ID. ErrNotFound при soft-deleted.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (queries.Discipline, error) {
	row, err := s.store.GetDisciplineByID(ctx, pgutil.PgUUID(id))
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.Discipline{}, ErrNotFound
		}
		return queries.Discipline{}, fmt.Errorf("get: %w", err)
	}
	return row, nil
}

// List — пагинированный список активных. limit ≤ 200 чтобы клиент не
// положил БД одним запросом.
func (s *Service) List(ctx context.Context, limit, offset int32) ([]queries.Discipline, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.store.ListDisciplines(ctx, queries.ListDisciplinesParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	total, err := s.store.CountDisciplines(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}
	return rows, total, nil
}

// Update применяет PATCH. Если меняется code — проверяем уникальность.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput, actorID uuid.UUID) (queries.Discipline, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return queries.Discipline{}, err
	}

	if in.Code != nil {
		newCode := strings.TrimSpace(*in.Code)
		if newCode == "" {
			return queries.Discipline{}, fmt.Errorf("%w: code не может быть пустым", ErrInvalidInput)
		}
		if !strings.EqualFold(newCode, current.Code) {
			if _, err := s.store.GetDisciplineByCode(ctx, newCode); err == nil {
				return queries.Discipline{}, ErrCodeTaken
			} else if !repo.IsNotFound(err) {
				return queries.Discipline{}, fmt.Errorf("check code: %w", err)
			}
		}
		in.Code = &newCode
	}
	if in.Name != nil {
		newName := strings.TrimSpace(*in.Name)
		if newName == "" {
			return queries.Discipline{}, fmt.Errorf("%w: name не может быть пустым", ErrInvalidInput)
		}
		if len([]rune(newName)) > 255 {
			return queries.Discipline{}, fmt.Errorf("%w: name не может превышать 255 символов", ErrInvalidInput)
		}
		in.Name = &newName
	}

	var updated queries.Discipline
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		row, err := q.UpdateDiscipline(ctx, queries.UpdateDisciplineParams{
			ID:          pgutil.PgUUID(id),
			Name:        in.Name,
			Code:        in.Code,
			Description: in.Description,
		})
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrNotFound
			}
			if repo.IsUniqueViolation(err, "idx_disciplines_name_lower") {
				return ErrNameTaken
			}
			if repo.IsUniqueViolation(err, "idx_disciplines_code_lower") {
				return ErrCodeTaken
			}
			return fmt.Errorf("update: %w", err)
		}
		updated = row

		before := disciplineFields(current)
		after := disciplineFields(row)
		if err := s.changelog.LogUpdatedTx(ctx, q, entityType, pgutil.UUID(row.ID).String(),
			before, after, actorID); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionUpdated,
			TargetType: entityType,
			TargetID:   pgutil.UUID(row.ID).String(),
		})
	})
	if err != nil {
		return queries.Discipline{}, err
	}
	return updated, nil
}

// SoftDelete помечает дисциплину удалённой. FK debts/retakes ссылаются
// на disciplines через RESTRICT — поэтому фактическое удаление невозможно,
// здесь только soft.
func (s *Service) SoftDelete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	current, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.SoftDeleteDiscipline(ctx, queries.SoftDeleteDisciplineParams{
			ID:        pgutil.PgUUID(id),
			DeletedBy: pgutil.PgUUID(actorID),
		}); err != nil {
			return fmt.Errorf("soft delete: %w", err)
		}
		if err := s.changelog.LogSoftDeletedTx(ctx, q, entityType, id.String(),
			disciplineFields(current), actorID); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionSoftDeleted,
			TargetType: entityType,
			TargetID:   id.String(),
		})
	})
}

// Restore возвращает soft-deleted запись. Возвращает ErrNotFound, если
// запись не существует вообще. Если запись уже активна — идемпотентно
// возвращает nil (восстанавливать нечего, но это не ошибка).
func (s *Service) Restore(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	exists, err := s.store.DisciplineExists(ctx, pgutil.PgUUID(id))
	if err != nil {
		return fmt.Errorf("check exists: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.RestoreDiscipline(ctx, pgutil.PgUUID(id)); err != nil {
			return fmt.Errorf("restore: %w", err)
		}
		if err := s.changelog.LogCustomTx(ctx, q, entityType, id.String(),
			changelog.ActionRestored, nil, nil, actorID); err != nil {
			return fmt.Errorf("changelog: %w", err)
		}
		return s.audit.LogTx(ctx, q, audit.Event{
			ActorID:    actorID,
			Action:     actionRestored,
			TargetType: entityType,
			TargetID:   id.String(),
		})
	})
}

// disciplineFields формирует JSON-friendly map полей для change_logs.
// Сознательно не включаем created_by/updated_at — это техническая
// мета, в истории "что изменилось" она шумит.
func disciplineFields(d queries.Discipline) changelog.Fields {
	f := changelog.Fields{
		"id":     pgutil.UUID(d.ID).String(),
		"name":   d.Name,
		"code":   d.Code,
		"source": d.Source,
	}
	if d.Description != nil {
		f["description"] = *d.Description
	}
	if d.ExternalID != nil {
		f["external_id"] = *d.ExternalID
	}
	return f
}
