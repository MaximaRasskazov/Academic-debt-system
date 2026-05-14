-- name: CreateRole :one
INSERT INTO roles (name, slug, description, level, is_system, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRoleByID :one
SELECT *
FROM roles
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetRoleBySlug :one
SELECT *
FROM roles
WHERE LOWER(slug) = LOWER($1)
  AND deleted_at IS NULL;

-- name: ListRoles :many
SELECT *
FROM roles
WHERE deleted_at IS NULL
ORDER BY level DESC, name ASC;

-- name: ListRolesIncludingDeleted :many
-- Для админ-интерфейса просмотра истории.
SELECT *
FROM roles
ORDER BY level DESC, name ASC;

-- name: UpdateRole :one
UPDATE roles
SET name        = COALESCE(sqlc.narg('name'),        name),
    description = COALESCE(sqlc.narg('description'), description),
    level       = COALESCE(sqlc.narg('level'),       level),
    updated_at  = NOW()
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteRole :exec
-- Системные роли (is_system=TRUE) защищаются от удаления на уровне
-- сервиса — здесь жёсткой проверки нет, чтобы запрос оставался
-- переиспользуемым.
UPDATE roles
SET deleted_at = NOW(),
    deleted_by = $2
WHERE id = $1
  AND deleted_at IS NULL;

-- name: RestoreRole :exec
UPDATE roles
SET deleted_at = NULL,
    deleted_by = NULL,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NOT NULL;
