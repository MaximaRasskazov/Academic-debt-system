-- name: CreateAuditEntry :one
INSERT INTO audit_log (actor_id, action, target_type, target_id, details, ip_address)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAuditByActor :many
SELECT *
FROM audit_log
WHERE actor_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListAuditByTarget :many
SELECT *
FROM audit_log
WHERE target_type = $1
  AND target_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListRecentAudit :many
SELECT *
FROM audit_log
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
