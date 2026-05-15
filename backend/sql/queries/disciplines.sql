-- name: CreateDiscipline :one
INSERT INTO disciplines (name, code, description, external_id, source, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetDisciplineByID :one
SELECT *
FROM disciplines
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetDisciplineByCode :one
SELECT *
FROM disciplines
WHERE LOWER(code) = LOWER($1)
  AND deleted_at IS NULL;

-- name: GetDisciplineByExternalID :one
-- Используется при синхронизации: если запись уже есть, делаем UPDATE,
-- иначе CREATE.
SELECT *
FROM disciplines
WHERE external_id = $1
  AND deleted_at IS NULL;

-- name: ListDisciplines :many
SELECT *
FROM disciplines
WHERE deleted_at IS NULL
ORDER BY name ASC
LIMIT $1 OFFSET $2;

-- name: CountDisciplines :one
SELECT COUNT(*) FROM disciplines WHERE deleted_at IS NULL;

-- name: UpdateDiscipline :one
UPDATE disciplines
SET name        = COALESCE(sqlc.narg('name'),        name),
    code        = COALESCE(sqlc.narg('code'),        code),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at  = NOW()
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDiscipline :exec
UPDATE disciplines
SET deleted_at = NOW(),
    deleted_by = $2
WHERE id = $1
  AND deleted_at IS NULL;

-- name: RestoreDiscipline :exec
UPDATE disciplines
SET deleted_at = NULL,
    deleted_by = NULL,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NOT NULL;
