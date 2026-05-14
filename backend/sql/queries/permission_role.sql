-- name: AttachPermissionToRole :one
INSERT INTO permission_role (role_id, permission_id, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DetachPermissionFromRole :exec
UPDATE permission_role
SET deleted_at = NOW(),
    deleted_by = $3
WHERE role_id = $1
  AND permission_id = $2
  AND deleted_at IS NULL;

-- name: ListPermissionsForRole :many
SELECT p.*
FROM permissions p
JOIN permission_role pr ON pr.permission_id = p.id
WHERE pr.role_id = $1
  AND pr.deleted_at IS NULL
  AND p.deleted_at IS NULL
ORDER BY p.slug ASC;
