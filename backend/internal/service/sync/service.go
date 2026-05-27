// Package sync реализует фоновую синхронизацию данных из внешнего
// эмулятора деканата в локальную БД.
//
// Алгоритм одного прогона (Sync):
//  1. Читаем last_synced_at из sync_state.
//  2. GET /api/v1/changes?since=<last_synced_at> — получаем список изменений.
//  3. Для каждого изменения вызываем соответствующий upsert в БД.
//  4. Обновляем last_synced_at = now().
//
// Повторный запуск с теми же данными идемпотентен: ON CONFLICT DO UPDATE
// в upsert-запросах гарантирует, что дублей не возникнет.
package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/emulator"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
)

// pgUUIDForce конвертирует uuid.UUID в pgtype.UUID с Valid=true даже для uuid.Nil,
// потому что system-пользователь (00000000-...) реально существует в БД.
func pgUUIDForce(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

const changesPageSize = 500

// Service синхронизирует данные из эмулятора.
type Service struct {
	store  *repo.Store
	client *emulator.Client
	// systemUserID — UUID пользователя-системы, от имени которого
	// записываются upsert'ы (created_by/issued_by в БД).
	systemUserID uuid.UUID
}

// New создаёт сервис. systemUserID — UUID существующего пользователя
// в БД (например, сид-пользователь "system" или admin).
func New(store *repo.Store, client *emulator.Client, systemUserID uuid.UUID) *Service {
	return &Service{store: store, client: client, systemUserID: systemUserID}
}

// Sync выполняет один цикл синхронизации: читает изменения из эмулятора
// и применяет их к локальной БД.
//
// Если хотя бы один запрос к эмулятору упал с rate-limit (429 после
// retry), last_synced_at НЕ сдвигается — при следующем тике эти change'ы
// придут снова, и мы успеем их применить. Обычные ошибки данных
// (битый student_id и т.п.) не блокируют сдвиг времени, иначе sync
// зависнет на грязной записи на стороне эмулятора.
func (s *Service) Sync(ctx context.Context) error {
	since, err := s.store.GetLastSyncedAt(ctx)
	if err != nil {
		return fmt.Errorf("sync: get last_synced_at: %w", err)
	}

	// При первом запуске (epoch) делаем полный импорт справочников.
	if since.Year() == 1970 {
		slog.Info("sync: первый запуск, полный импорт дисциплин")
		if err := s.importAllDisciplines(ctx); err != nil {
			slog.Warn("sync: ошибка импорта дисциплин", "err", err)
		}
	}

	changes, err := s.client.GetChanges(ctx, since, changesPageSize)
	if err != nil {
		return fmt.Errorf("sync: get changes: %w", err)
	}

	if len(changes) == 0 {
		slog.Info("sync: нет новых изменений", "since", since)
		if err := s.store.SetLastSyncedAt(ctx, time.Now().UTC()); err != nil {
			return fmt.Errorf("sync: set last_synced_at: %w", err)
		}
		return nil
	}

	slog.Info("sync: получены изменения", "count", len(changes), "since", since)

	var (
		errs            []string
		rateLimitedHits int
	)
	for _, ch := range changes {
		if err := s.applyChange(ctx, ch); err != nil {
			slog.Warn("sync: ошибка применения изменения",
				"entity_type", ch.EntityType,
				"entity_id", ch.EntityID,
				"err", err,
			)
			errs = append(errs, fmt.Sprintf("%s/%s: %v", ch.EntityType, ch.EntityID, err))
			if errors.Is(err, emulator.ErrRateLimited) {
				rateLimitedHits++
			}
		}
	}

	// Если эмулятор throttle-ил хотя бы один запрос, не двигаем
	// last_synced_at — иначе потеряем те change'ы, которые не успели
	// применить (при следующем тике since уже их перескочит).
	if rateLimitedHits > 0 {
		slog.Warn("sync: rate limited, last_synced_at не сдвигается",
			"hits", rateLimitedHits, "since", since)
		return fmt.Errorf("sync: rate limited (%d запросов 429), повтор на следующем тике", rateLimitedHits)
	}

	if err := s.store.SetLastSyncedAt(ctx, time.Now().UTC()); err != nil {
		return fmt.Errorf("sync: set last_synced_at: %w", err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("sync: %d ошибок: %s", len(errs), strings.Join(errs, "; "))
	}
	return nil
}

// LastSyncedAt возвращает время последней успешной синхронизации.
func (s *Service) LastSyncedAt(ctx context.Context) (time.Time, error) {
	return s.store.GetLastSyncedAt(ctx)
}

// applyChange обрабатывает одно изменение из эмулятора.
func (s *Service) applyChange(ctx context.Context, ch emulator.ChangeEntry) error {
	switch ch.EntityType {
	case "discipline":
		return s.syncDiscipline(ctx, ch.EntityID)
	case "debt":
		return s.syncDebt(ctx, ch.EntityID)
	default:
		// Неизвестный тип — пропускаем без ошибки.
		return nil
	}
}

// importAllDisciplines постранично тянет все дисциплины из эмулятора и делает upsert.
func (s *Service) importAllDisciplines(ctx context.Context) error {
	const pageSize = 100
	page := 1
	total := 0
	for {
		items, meta, err := s.client.ListDisciplines(ctx, page, pageSize)
		if err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}
		for _, d := range items {
			code := d.Code
			if code == "" {
				code = "EXT-" + d.ID
			}
			if err := s.store.UpsertDisciplineFromSync(ctx, repo.UpsertDisciplineParams{
				Name:         d.Name,
				Code:         code,
				Description:  d.Description,
				ExternalID:   d.ID,
				SystemUserID: pgUUIDForce(s.systemUserID),
			}); err != nil {
				slog.Warn("sync: upsert discipline failed", "id", d.ID, "err", err)
			}
		}
		total += len(items)
		if page >= meta.TotalPages {
			break
		}
		page++
	}
	slog.Info("sync: дисциплины импортированы", "count", total)
	return nil
}

// syncDiscipline тянет дисциплину из эмулятора и делает upsert в БД.
func (s *Service) syncDiscipline(ctx context.Context, externalID string) error {
	d, err := s.client.GetDiscipline(ctx, externalID)
	if err != nil {
		if errors.Is(err, emulator.ErrNotFound) {
			return nil
		}
		return err
	}

	code := d.Code
	if code == "" {
		// Если код не задан — используем ID как fallback.
		code = "EXT-" + externalID
	}

	return s.store.UpsertDisciplineFromSync(ctx, repo.UpsertDisciplineParams{
		Name:         d.Name,
		Code:         code,
		Description:  d.Description,
		ExternalID:   externalID,
		SystemUserID: pgUUIDForce(s.systemUserID),
	})
}

// syncDebt тянет долг из эмулятора и делает upsert в БД.
// Студент и дисциплина должны уже быть в нашей БД (приходят раньше
// через соответствующие change-события).
func (s *Service) syncDebt(ctx context.Context, externalID string) error {
	d, err := s.client.GetDebt(ctx, externalID)
	if err != nil {
		if errors.Is(err, emulator.ErrNotFound) {
			return nil
		}
		return err
	}

	studentID, err := uuid.Parse(d.StudentID)
	if err != nil {
		return fmt.Errorf("parse student_id %q: %w", d.StudentID, err)
	}
	disciplineID, err := uuid.Parse(d.DisciplineID)
	if err != nil {
		return fmt.Errorf("parse discipline_id %q: %w", d.DisciplineID, err)
	}
	teacherID, err := uuid.Parse(d.TeacherID)
	if err != nil {
		// Преподаватель может быть не задан — используем системного пользователя.
		teacherID = s.systemUserID
	}

	status := d.Status
	if status == "" {
		status = "open"
	}

	var finalGrade *int32
	var gradedAt pgtype.Timestamptz
	var gradedBy pgtype.UUID

	if d.Grade != nil && status == "graded" {
		// Эмулятор передаёт оценку в диапазоне 2..5 — переполнение невозможно,
		// но gosec требует явного clamp'а для int→int32 conversion.
		gradeVal := *d.Grade
		if gradeVal < 2 || gradeVal > 5 {
			gradeVal = 2
		}
		g := int32(gradeVal) //nolint:gosec
		finalGrade = &g
		gradedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
		gradedBy = pgUUIDForce(teacherID)
	}

	return s.store.UpsertDebtFromSync(ctx, repo.UpsertDebtParams{
		StudentID:    pgutil.PgUUID(studentID),
		DisciplineID: pgutil.PgUUID(disciplineID),
		IssuedBy:     pgUUIDForce(teacherID),
		ExternalID:   externalID,
		Notes:        d.Notes,
		Status:       status,
		FinalGrade:   finalGrade,
		GradedAt:     gradedAt,
		GradedBy:     gradedBy,
	})
}
