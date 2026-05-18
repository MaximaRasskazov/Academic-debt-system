-- name: CreateRetake :one
INSERT INTO retakes (discipline_id, kind, min_teachers, building, room, scheduled_at, duration_minutes, created_by, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetRetakeByID :one
SELECT *
FROM retakes
WHERE id = $1
  AND deleted_at IS NULL;

-- name: ListRetakes :many
-- Деканат: общий список пересдач, с фильтром по статусу при необходимости.
-- sqlc.narg('status') NULL означает "все статусы".
SELECT *
FROM retakes
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
ORDER BY scheduled_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListRetakesForUser :many
-- Все пересдачи, в которых пользователь — участник (любой kind).
-- Используется в личном кабинете студента и преподавателя
-- (permission retakes.view.own).
SELECT DISTINCT r.*
FROM retakes r
JOIN retake_participants rp ON rp.retake_id = r.id
WHERE rp.user_id = $1
  AND r.deleted_at IS NULL
ORDER BY r.scheduled_at DESC;

-- name: ListRetakesInPeriod :many
-- Для сводного отчёта деканата за период: только проведённые.
SELECT *
FROM retakes
WHERE deleted_at IS NULL
  AND status = 'completed'
  AND completed_at >= sqlc.arg('from')
  AND completed_at <  sqlc.arg('to')
ORDER BY completed_at ASC;

-- name: UpdateRetakeSchedule :one
-- Деканат меняет время/место/продолжительность пересдачи (например,
-- по итогам одобренной change-request заявки от преподавателя).
UPDATE retakes
SET building         = COALESCE(sqlc.narg('building'),         building),
    room             = COALESCE(sqlc.narg('room'),             room),
    scheduled_at     = COALESCE(sqlc.narg('scheduled_at'),     scheduled_at),
    duration_minutes = COALESCE(sqlc.narg('duration_minutes'), duration_minutes),
    notes            = COALESCE(sqlc.narg('notes'),            notes),
    updated_at       = NOW()
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL
RETURNING *;

-- name: MarkRetakeInProgress :exec
-- Шедулер переводит пересдачу в in_progress по достижении scheduled_at.
UPDATE retakes
SET status     = 'in_progress',
    updated_at = NOW()
WHERE id = $1
  AND status = 'scheduled'
  AND deleted_at IS NULL;

-- name: MarkRetakeCompleted :exec
-- Шедулер переводит в completed по истечении scheduled_at + duration.
UPDATE retakes
SET status       = 'completed',
    completed_at = NOW(),
    updated_at   = NOW()
WHERE id = $1
  AND status IN ('scheduled', 'in_progress')
  AND deleted_at IS NULL;

-- name: ListRetakesToStart :many
-- Шедулер на каждом тике: пересдачи, которые пора стартовать
-- (scheduled_at прошёл, статус всё ещё scheduled).
SELECT *
FROM retakes
WHERE status = 'scheduled'
  AND scheduled_at <= NOW()
  AND deleted_at IS NULL
ORDER BY scheduled_at ASC;

-- name: ListRetakesToFinish :many
-- Шедулер: пересдачи, которые пора завершать (scheduled_at + duration < NOW()).
SELECT *
FROM retakes
WHERE status IN ('scheduled', 'in_progress')
  AND (scheduled_at + (duration_minutes || ' minutes')::interval) <= NOW()
  AND deleted_at IS NULL
ORDER BY scheduled_at ASC;

-- name: CancelRetake :exec
UPDATE retakes
SET status     = 'cancelled',
    updated_at = NOW()
WHERE id = $1
  AND status IN ('scheduled', 'in_progress')
  AND deleted_at IS NULL;

-- name: SoftDeleteRetake :exec
UPDATE retakes
SET deleted_at = NOW(),
    deleted_by = $2,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;
