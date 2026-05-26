// Package rbac реализует проверку прав доступа и управление ролями.
//
// Два сценария использования:
//
//  1. HTTP-middleware на каждом защищённом запросе вызывает HasPermission
//     для конкретного permission-slug.
//  2. Админ-handler'ы вызывают AssignRole / RevokeRole для назначения
//     ролей другим пользователям. Эти операции защищены от privilege
//     escalation: actor не может выдать роль с level >= собственного
//     максимального level.
package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// PermAssignRoles — slug разрешения, требуемого для выдачи ролей.
// Дублируется здесь как константа, чтобы handler-/middleware-слои
// могли явно ссылаться на него и опечатки ловились компилятором.
const PermAssignRoles = "roles.assign"

// Sentinel-ошибки сервиса.
var (
	ErrRoleNotFound        = errors.New("rbac: роль не найдена")
	ErrUserNotFound        = errors.New("rbac: пользователь не найден")
	ErrPermissionDenied    = errors.New("rbac: недостаточно прав")
	ErrPrivilegeEscalation = errors.New("rbac: попытка выдать роль выше своего уровня")
)

type Service struct {
	store *repo.Store
}

func New(store *repo.Store) *Service {
	return &Service{store: store}
}

// HasPermission — основная горячая операция. Возвращает true, если
// у userID есть активная привязка к роли, которая содержит permission
// с указанным slug. Случай "пользователя нет" — это false, не ошибка
// (вызов с просроченным токеном уже отсёкся в auth-middleware).
func (s *Service) HasPermission(ctx context.Context, userID uuid.UUID, slug string) (bool, error) {
	ok, err := s.store.UserHasPermission(ctx, queries.UserHasPermissionParams{
		UserID: pgutil.PgUUID(userID),
		Lower:  slug,
	})
	if err != nil {
		return false, fmt.Errorf("user has permission: %w", err)
	}
	return ok, nil
}

// AssignRole выдаёт роль roleSlug пользователю targetID от имени
// actorID. Проверяет, что:
//   - actor имеет roles.assign;
//   - level назначаемой роли строго меньше максимального level actor'а
//     (нельзя выдать роль равную или выше собственной — иначе можно
//     повысить себя и других до своего уровня и сравняться с админом).
//
// Запись добавляется в role_user и параллельно в audit_log.
// Если такая привязка уже есть (UNIQUE-индекс) — возвращается ошибка
// со стороны БД; идемпотентность здесь намеренно не добавляем, чтобы
// клиент явно знал, что запрос был дублем.
func (s *Service) AssignRole(ctx context.Context, actorID, targetID uuid.UUID, roleSlug string) error {
	role, err := s.lookupRole(ctx, roleSlug)
	if err != nil {
		return err
	}
	if err := s.requireCanManage(ctx, actorID, role); err != nil {
		return err
	}

	pgActor := pgutil.PgUUID(actorID)
	pgTarget := pgutil.PgUUID(targetID)

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if _, err := q.AttachRoleToUser(ctx, queries.AttachRoleToUserParams{
			UserID:    pgTarget,
			RoleID:    role.ID,
			CreatedBy: pgActor,
		}); err != nil {
			return fmt.Errorf("attach role: %w", err)
		}
		details := mustJSON(map[string]any{"role_slug": role.Slug, "role_id": pgutil.UUID(role.ID).String()})
		targetStr := targetID.String()
		_, err := q.CreateAuditEntry(ctx, queries.CreateAuditEntryParams{
			ActorID:    pgActor,
			Action:     "role.assign",
			TargetType: "user",
			TargetID:   &targetStr,
			Details:    details,
		})
		return err
	})
}

// RevokeRole — обратная операция. Та же проверка уровня: отзывать
// роль выше собственного level тоже нельзя (это в каком-то смысле
// смягчение, но симметричное правило проще объяснить и реализовать).
func (s *Service) RevokeRole(ctx context.Context, actorID, targetID uuid.UUID, roleSlug string) error {
	role, err := s.lookupRole(ctx, roleSlug)
	if err != nil {
		return err
	}
	if err := s.requireCanManage(ctx, actorID, role); err != nil {
		return err
	}

	pgActor := pgutil.PgUUID(actorID)
	pgTarget := pgutil.PgUUID(targetID)

	return s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.DetachRoleFromUser(ctx, queries.DetachRoleFromUserParams{
			UserID:    pgTarget,
			RoleID:    role.ID,
			DeletedBy: pgActor,
		}); err != nil {
			return fmt.Errorf("detach role: %w", err)
		}
		details := mustJSON(map[string]any{"role_slug": role.Slug})
		targetStr := targetID.String()
		_, err := q.CreateAuditEntry(ctx, queries.CreateAuditEntryParams{
			ActorID:    pgActor,
			Action:     "role.revoke",
			TargetType: "user",
			TargetID:   &targetStr,
			Details:    details,
		})
		return err
	})
}

// MaxLevel возвращает максимальный level среди активных ролей
// пользователя. Если ролей нет — 0.
func (s *Service) MaxLevel(ctx context.Context, userID uuid.UUID) (int32, error) {
	roles, err := s.store.ListRolesForUser(ctx, pgutil.PgUUID(userID))
	if err != nil {
		return 0, fmt.Errorf("list roles: %w", err)
	}
	var maxLevel int32
	for _, r := range roles {
		if r.Level > maxLevel {
			maxLevel = r.Level
		}
	}
	return maxLevel, nil
}

func (s *Service) lookupRole(ctx context.Context, slug string) (queries.Role, error) {
	role, err := s.store.GetRoleBySlug(ctx, slug)
	if err != nil {
		if repo.IsNotFound(err) {
			return queries.Role{}, ErrRoleNotFound
		}
		return queries.Role{}, fmt.Errorf("get role: %w", err)
	}
	return role, nil
}

// requireCanManage проверяет, что actor может работать с ролью role.
// Условия: actor имеет roles.assign И actor.maxLevel > role.level.
func (s *Service) requireCanManage(ctx context.Context, actorID uuid.UUID, role queries.Role) error {
	hasPerm, err := s.HasPermission(ctx, actorID, PermAssignRoles)
	if err != nil {
		return err
	}
	if !hasPerm {
		return ErrPermissionDenied
	}
	level, err := s.MaxLevel(ctx, actorID)
	if err != nil {
		return err
	}
	if level <= role.Level {
		return ErrPrivilegeEscalation
	}
	return nil
}

// ListRoles возвращает все активные роли системы.
func (s *Service) ListRoles(ctx context.Context) ([]queries.Role, error) {
	roles, err := s.store.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return roles, nil
}

// ListPermissions возвращает все активные права системы.
func (s *Service) ListPermissions(ctx context.Context) ([]queries.Permission, error) {
	perms, err := s.store.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	return perms, nil
}

// ListUserPermissions возвращает плоский список slug'ов прав пользователя.
func (s *Service) ListUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	slugs, err := s.store.ListPermissionsForUser(ctx, pgutil.PgUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("list user permissions: %w", err)
	}
	return slugs, nil
}

// ListUserRoles возвращает активные роли пользователя.
func (s *Service) ListUserRoles(ctx context.Context, userID uuid.UUID) ([]queries.Role, error) {
	roles, err := s.store.ListRolesForUser(ctx, pgutil.PgUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}
	return roles, nil
}

// mustJSON маршалит payload для audit_log.details. Падать не на чем —
// мы подаём только структуры, в которых нет каналов/функций.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Errorf("rbac: marshal audit details: %w", err))
	}
	return b
}
