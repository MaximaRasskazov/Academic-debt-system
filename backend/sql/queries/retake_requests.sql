-- name: CreateRetakeRequest :one
INSERT INTO retake_requests (requested_by, payload)
VALUES ($1, $2)
RETURNING *;

-- name: GetRetakeRequestByID :one
SELECT *
FROM retake_requests
WHERE id = $1;

-- name: ListPendingRetakeRequests :many
-- Деканат: входящие заявки на рассмотрение.
SELECT *
FROM retake_requests
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1 OFFSET $2;

-- name: ListRetakeRequestsForTeacher :many
-- История заявок преподавателя (его собственные).
SELECT *
FROM retake_requests
WHERE requested_by = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ApproveRetakeRequest :one
-- created_retake_id заполняется в сервисе уже после q.CreateRetake
-- в той же транзакции — поэтому здесь это обязательный параметр.
UPDATE retake_requests
SET status            = 'approved',
    reviewed_by       = sqlc.arg('reviewed_by')::uuid,
    reviewed_at       = NOW(),
    decision_reason   = sqlc.narg('decision_reason')::text,
    created_retake_id = sqlc.arg('created_retake_id')::uuid,
    updated_at        = NOW()
WHERE id = sqlc.arg('id')::uuid
  AND status = 'pending'
RETURNING *;

-- name: RejectRetakeRequest :one
-- При rejected decision_reason обязателен — CHECK-constraint в таблице
-- гарантирует это даже если сервис забудет передать.
UPDATE retake_requests
SET status          = 'rejected',
    reviewed_by     = sqlc.arg('reviewed_by')::uuid,
    reviewed_at     = NOW(),
    decision_reason = sqlc.arg('decision_reason')::text,
    updated_at      = NOW()
WHERE id = sqlc.arg('id')::uuid
  AND status = 'pending'
RETURNING *;
