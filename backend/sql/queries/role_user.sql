-- name: AttachRoleToUser :one
-- Идемпотентно: при повторной выдаче той же роли SoftDelete ничего не
-- меняет, INSERT даст конфликт по уникальному индексу. Идемпотентность
-- обеспечивается на уровне сервиса (сначала проверка существования).
INSERT INTO role_user (user_id, role_id, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DetachRoleFromUser :exec
UPDATE role_user
SET deleted_at = NOW(),
    deleted_by = $3
WHERE user_id = $1
  AND role_id = $2
  AND deleted_at IS NULL;

-- name: ListRolesForUser :many
SELECT r.*
FROM roles r
JOIN role_user ru ON ru.role_id = r.id
WHERE ru.user_id = $1
  AND ru.deleted_at IS NULL
  AND r.deleted_at IS NULL
ORDER BY r.level DESC;

-- name: ListUsersInRole :many
SELECT u.*
FROM users u
JOIN role_user ru ON ru.user_id = u.id
WHERE ru.role_id = $1
  AND ru.deleted_at IS NULL
ORDER BY u.last_name ASC, u.first_name ASC;

-- name: HasRole :one
-- Проверка наличия конкретной роли у пользователя.
SELECT EXISTS (
    SELECT 1
    FROM role_user ru
    JOIN roles r ON r.id = ru.role_id
    WHERE ru.user_id = $1
      AND LOWER(r.slug) = LOWER($2)
      AND ru.deleted_at IS NULL
      AND r.deleted_at IS NULL
) AS has_role;

-- name: ListPermissionsForUser :many
-- Возвращает все активные slug'и разрешений, доступных пользователю
-- через его роли. Дубли убираются DISTINCT, потому что один permission
-- может быть прикреплён к нескольким ролям пользователя.
SELECT DISTINCT p.slug
FROM role_user ru
JOIN permission_role pr ON pr.role_id = ru.role_id
JOIN permissions p ON p.id = pr.permission_id
JOIN roles r ON r.id = ru.role_id
WHERE ru.user_id = $1
  AND ru.deleted_at IS NULL
  AND pr.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND r.deleted_at IS NULL
ORDER BY p.slug ASC;

-- name: UserHasPermission :one
-- Точечная проверка на конкретный permission по slug. Используется в
-- RBAC-middleware на каждом защищённом запросе.
SELECT EXISTS (
    SELECT 1
    FROM role_user ru
    JOIN permission_role pr ON pr.role_id = ru.role_id
    JOIN permissions p ON p.id = pr.permission_id
    JOIN roles r ON r.id = ru.role_id
    WHERE ru.user_id = $1
      AND LOWER(p.slug) = LOWER($2)
      AND ru.deleted_at IS NULL
      AND pr.deleted_at IS NULL
      AND p.deleted_at IS NULL
      AND r.deleted_at IS NULL
) AS has_permission;
