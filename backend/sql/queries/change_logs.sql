-- name: CreateChangeLog :one
INSERT INTO change_logs (entity_type, entity_id, action, before, after, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetChangeLogByID :one
SELECT *
FROM change_logs
WHERE id = $1;

-- name: ListChangeLogsForEntity :many
-- История изменений конкретной сущности — лента "что менялось у этого
-- долга / пересдачи / роли". Используется при выводе story-страницы.
SELECT *
FROM change_logs
WHERE entity_type = $1
  AND entity_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListChangeLogsByEntityType :many
SELECT *
FROM change_logs
WHERE entity_type = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListChangeLogsByAuthor :many
SELECT *
FROM change_logs
WHERE created_by = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
