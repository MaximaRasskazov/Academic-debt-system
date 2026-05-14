-- name: CreatePermission :one
INSERT INTO permissions (name, slug, description, is_system, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPermissionByID :one
SELECT *
FROM permissions
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetPermissionBySlug :one
SELECT *
FROM permissions
WHERE LOWER(slug) = LOWER($1)
  AND deleted_at IS NULL;

-- name: ListPermissions :many
SELECT *
FROM permissions
WHERE deleted_at IS NULL
ORDER BY slug ASC;

-- name: UpdatePermission :one
UPDATE permissions
SET name        = COALESCE(sqlc.narg('name'),        name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at  = NOW()
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeletePermission :exec
UPDATE permissions
SET deleted_at = NOW(),
    deleted_by = $2
WHERE id = $1
  AND deleted_at IS NULL;

-- name: RestorePermission :exec
UPDATE permissions
SET deleted_at = NULL,
    deleted_by = NULL,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NOT NULL;
