// Package sync реализует фоновую синхронизацию данных из внешнего
// эмулятора деканата в локальную БД.
//
// Алгоритм одного прогона (Sync):
//  1. Читаем last_synced_at из sync_state.
//  2. Постранично читаем GET /api/v1/changes?since=<last_synced_at> пока has_more=false.
//  3. Для каждого изменения вызываем соответствующий upsert в БД.
//  4. Обновляем last_synced_at = now().
//
// При первом запуске (epoch) делаем полный импорт: дисциплины, затем
// аккаунты (студенты и преподаватели), затем долги из delta.
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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/emulator"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
)

// pgUUIDForce конвертирует pgtype.UUID в pgtype.UUID с Valid=true —
// используется для system-пользователя (uuid.Nil), который реально есть в БД.
func pgUUIDForce(id pgtype.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id.Bytes, Valid: true}
}

const changesPageSize = 500

// Service синхронизирует данные из эмулятора.
type Service struct {
	store        *repo.Store
	client       *emulator.Client
	systemUserID pgtype.UUID // UUID пользователя-системы (created_by в upsert'ах)
}

// New создаёт сервис.
func New(store *repo.Store, client *emulator.Client, systemUserID pgtype.UUID) *Service {
	return &Service{store: store, client: client, systemUserID: systemUserID}
}

// Sync выполняет один цикл синхронизации: читает изменения из эмулятора
// и применяет их к локальной БД.
func (s *Service) Sync(ctx context.Context) error {
	since, err := s.store.GetLastSyncedAt(ctx)
	if err != nil {
		return fmt.Errorf("sync: get last_synced_at: %w", err)
	}

	// При первом запуске делаем полный импорт справочников.
	if since.Year() == 1970 {
		slog.Info("sync: первый запуск, полный импорт")
		if err := s.importAllDisciplines(ctx); err != nil {
			slog.Warn("sync: ошибка импорта дисциплин", "err", err)
		}
		if err := s.importAllAccounts(ctx, "student"); err != nil {
			slog.Warn("sync: ошибка импорта студентов", "err", err)
		}
		if err := s.importAllAccounts(ctx, "teacher"); err != nil {
			slog.Warn("sync: ошибка импорта преподавателей", "err", err)
		}
		if err := s.importAllDebts(ctx); err != nil {
			slog.Warn("sync: ошибка импорта долгов", "err", err)
		}
	}

	// Читаем все страницы изменений.
	var (
		allChanges      []emulator.ChangeEntry
		rateLimitedHits int
	)

	currentSince := since
	for {
		page, err := s.client.GetChangesPage(ctx, currentSince, changesPageSize)
		if err != nil {
			if errors.Is(err, emulator.ErrRateLimited) {
				return fmt.Errorf("sync: get changes rate limited, повтор на следующем тике")
			}
			return fmt.Errorf("sync: get changes: %w", err)
		}
		allChanges = append(allChanges, page.Entries...)
		if !page.HasMore {
			break
		}
		// next_since — строка RFC3339 от эмулятора; парсим как курсор следующей страницы.
		nextT, parseErr := time.Parse(time.RFC3339, page.NextSince)
		if parseErr != nil || nextT.Equal(currentSince) {
			// Не смогли разобрать курсор — выходим чтобы не зациклиться.
			break
		}
		currentSince = nextT
	}

	if len(allChanges) == 0 {
		slog.Info("sync: нет новых изменений", "since", since)
		return s.store.SetLastSyncedAt(ctx, time.Now().UTC())
	}

	slog.Info("sync: получены изменения", "count", len(allChanges), "since", since)

	var errs []string
	for _, ch := range allChanges {
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

	// Если был rate limit — не двигаем last_synced_at, повторим при следующем тике.
	if rateLimitedHits > 0 {
		slog.Warn("sync: rate limited, last_synced_at не сдвигается",
			"hits", rateLimitedHits)
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
	case "account":
		return s.syncAccount(ctx, ch.EntityID)
	case "debt":
		return s.syncDebt(ctx, ch.EntityID)
	default:
		// Неизвестный тип (group, etc.) — пропускаем без ошибки.
		return nil
	}
}

// ── Дисциплины ────────────────────────────────────────────────────────────────

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
				SystemUserID: s.systemUserID,
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
		code = "EXT-" + externalID
	}
	return s.store.UpsertDisciplineFromSync(ctx, repo.UpsertDisciplineParams{
		Name:         d.Name,
		Code:         code,
		Description:  d.Description,
		ExternalID:   externalID,
		SystemUserID: s.systemUserID,
	})
}

// ── Аккаунты (студенты / преподаватели) ──────────────────────────────────────

// importAllAccounts постранично импортирует аккаунты нужной роли.
// role: "student" | "teacher"
func (s *Service) importAllAccounts(ctx context.Context, role string) error {
	const pageSize = 100
	page := 1
	total := 0
	for {
		items, meta, err := s.client.ListAccounts(ctx, role, page, pageSize)
		if err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}
		for _, a := range items {
			if err := s.upsertAccount(ctx, a); err != nil {
				slog.Warn("sync: upsert account failed", "id", a.ID, "role", a.Role, "err", err)
			}
		}
		total += len(items)
		if page >= meta.TotalPages {
			break
		}
		page++
	}
	slog.Info("sync: аккаунты импортированы", "role", role, "count", total)
	return nil
}

// syncAccount тянет один аккаунт из эмулятора и делает upsert пользователя + роль.
func (s *Service) syncAccount(ctx context.Context, externalID string) error {
	a, err := s.client.GetAccount(ctx, externalID)
	if err != nil {
		if errors.Is(err, emulator.ErrNotFound) {
			return nil
		}
		return err
	}
	return s.upsertAccount(ctx, *a)
}

// upsertAccount вставляет/обновляет пользователя и назначает ему роль.
func (s *Service) upsertAccount(ctx context.Context, a emulator.AccountDTO) error {
	// Пропускаем неактивных пользователей.
	if a.Status == "inactive" || a.Status == "blocked" {
		return nil
	}
	// Пропускаем роли, которые не относятся к student/teacher.
	roleSlug := normalizeRole(a.Role)
	if roleSlug == "" {
		return nil
	}

	// linked_entity_id — это ID студента/преподавателя, который используется
	// в долгах (debt.student_id). Храним его как external_id пользователя,
	// чтобы syncDebt мог найти пользователя по debt.student_id.
	extID := a.LinkedEntityID
	if extID == "" {
		extID = a.ID // fallback на ID аккаунта если linked_entity_id не заполнен
	}
	userID, err := s.store.UpsertUserFromSync(ctx, repo.UpsertUserParams{
		Email:        a.Email,
		FirstName:    a.FirstName,
		LastName:     a.LastName,
		MiddleName:   a.MiddleName,
		ExternalID:   extID,
		PasswordHash: a.PasswordHash,
	})
	if err != nil {
		return fmt.Errorf("upsert user %s: %w", a.ID, err)
	}

	if err := s.store.AssignRoleFromSync(ctx, userID, s.systemUserID, roleSlug); err != nil {
		return fmt.Errorf("assign role %s to %s: %w", roleSlug, a.ID, err)
	}
	return nil
}

// normalizeRole приводит роль из эмулятора к slug нашей системы.
func normalizeRole(emulatorRole string) string {
	switch strings.ToLower(emulatorRole) {
	case "student":
		return "student"
	case "teacher":
		return "teacher"
	default:
		return ""
	}
}

// ── Долги ─────────────────────────────────────────────────────────────────────

// syncDebt тянет долг из эмулятора и делает upsert в БД.
func (s *Service) syncDebt(ctx context.Context, externalID string) error {
	d, err := s.client.GetDebt(ctx, externalID)
	if err != nil {
		if errors.Is(err, emulator.ErrNotFound) {
			return nil
		}
		return err
	}
	return s.applyDebt(ctx, *d, externalID)
}

// applyDebt выполняет upsert долга из DTO в БД.
// Вынесено отдельно чтобы importAllDebts мог использовать данные из списка
// напрямую, без повторного запроса к /api/v1/debts/{id}.
func (s *Service) applyDebt(ctx context.Context, d emulator.DebtDTO, externalID string) error {
	// Ищем дисциплину по external_id.
	disciplineID, err := s.store.GetDisciplineIDByExternalID(ctx, d.DisciplineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("дисциплина %s ещё не синхронизирована", d.DisciplineID)
		}
		return fmt.Errorf("get discipline: %w", err)
	}

	// Ищем студента по external_id (= linked_entity_id аккаунта).
	studentID, err := s.store.GetUserIDByExternalID(ctx, d.StudentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("студент %s ещё не синхронизирован", d.StudentID)
		}
		return fmt.Errorf("get student: %w", err)
	}

	// Преподаватель опционален — используем системного пользователя как fallback.
	issuedBy := s.systemUserID
	if d.TeacherID != "" {
		if teacherID, err := s.store.GetUserIDByExternalID(ctx, d.TeacherID); err == nil {
			issuedBy = teacherID
		}
	}

	status := d.Status
	if status == "" {
		status = "open"
	}

	var finalGrade *int32
	var gradedAt pgtype.Timestamptz
	var gradedBy pgtype.UUID

	if d.Grade != nil && status == "graded" {
		gradeVal := *d.Grade
		if gradeVal < 2 || gradeVal > 5 {
			gradeVal = 2
		}
		g := int32(gradeVal) //nolint:gosec
		finalGrade = &g
		gradedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
		gradedBy = pgUUIDForce(issuedBy)
	}

	return s.store.UpsertDebtFromSync(ctx, repo.UpsertDebtParams{
		StudentID:    studentID,
		DisciplineID: disciplineID,
		IssuedBy:     pgUUIDForce(issuedBy),
		ExternalID:   externalID,
		Notes:        d.Notes,
		Status:       status,
		FinalGrade:   finalGrade,
		GradedAt:     gradedAt,
		GradedBy:     gradedBy,
	})
}

// importAllDebts постранично импортирует все долги из эмулятора напрямую
// из списка, без повторных запросов по отдельным ID.
func (s *Service) importAllDebts(ctx context.Context) error {
	const pageSize = 100
	page := 1
	total := 0
	errCount := 0
	for {
		items, meta, err := s.client.ListDebts(ctx, page, pageSize)
		if err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}
		for _, d := range items {
			if err := s.applyDebt(ctx, d, d.ID); err != nil {
				slog.Warn("sync: upsert debt failed", "id", d.ID, "err", err)
				errCount++
			}
		}
		total += len(items)
		if page >= meta.TotalPages {
			break
		}
		page++
	}
	slog.Info("sync: долги импортированы", "total", total, "errors", errCount)
	return nil
}
