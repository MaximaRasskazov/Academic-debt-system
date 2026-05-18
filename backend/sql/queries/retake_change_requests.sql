-- name: CreateRetakeChangeRequest :one
INSERT INTO retake_change_requests (retake_id, requested_by, requested_changes, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRetakeChangeRequestByID :one
SELECT *
FROM retake_change_requests
WHERE id = $1;

-- name: ListPendingRetakeChangeRequests :many
-- Деканат: входящие заявки на рассмотрение.
SELECT *
FROM retake_change_requests
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1 OFFSET $2;

-- name: ListRetakeChangeRequestsForRetake :many
SELECT *
FROM retake_change_requests
WHERE retake_id = $1
ORDER BY created_at DESC;

-- name: ListRetakeChangeRequestsForTeacher :many
-- История заявок преподавателя.
SELECT *
FROM retake_change_requests
WHERE requested_by = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ApproveRetakeChangeRequest :one
UPDATE retake_change_requests
SET status          = 'approved',
    reviewed_by     = sqlc.arg('reviewed_by')::uuid,
    reviewed_at     = NOW(),
    decision_reason = sqlc.narg('decision_reason')::text,
    updated_at      = NOW()
WHERE id = sqlc.arg('id')::uuid
  AND status = 'pending'
RETURNING *;

-- name: RejectRetakeChangeRequest :one
-- При rejected decision_reason обязателен — CHECK-constraint в таблице
-- гарантирует это даже если сервис забудет передать.
UPDATE retake_change_requests
SET status          = 'rejected',
    reviewed_by     = sqlc.arg('reviewed_by')::uuid,
    reviewed_at     = NOW(),
    decision_reason = sqlc.arg('decision_reason')::text,
    updated_at      = NOW()
WHERE id = sqlc.arg('id')::uuid
  AND status = 'pending'
RETURNING *;
