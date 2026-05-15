-- name: CreateTeacherRoleRequest :one
INSERT INTO teacher_role_requests (requested_by, reason)
VALUES ($1, $2)
RETURNING *;

-- name: GetTeacherRoleRequestByID :one
SELECT *
FROM teacher_role_requests
WHERE id = $1;

-- name: GetPendingTeacherRoleRequestForUser :one
-- Перед созданием новой заявки сервис проверяет наличие текущей
-- pending — иначе UNIQUE-индекс idx_teacher_role_requests_unique_pending
-- даст ошибку БД.
SELECT *
FROM teacher_role_requests
WHERE requested_by = $1
  AND status = 'pending';

-- name: ListPendingTeacherRoleRequests :many
-- Деканат: входящие на рассмотрение.
SELECT *
FROM teacher_role_requests
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1 OFFSET $2;

-- name: ListTeacherRoleRequestsForUser :many
SELECT *
FROM teacher_role_requests
WHERE requested_by = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ApproveTeacherRoleRequest :one
-- Сервис вызывает в одной транзакции с AttachRoleToUser(teacher).
UPDATE teacher_role_requests
SET status          = 'approved',
    reviewed_by     = sqlc.arg('reviewed_by')::uuid,
    reviewed_at     = NOW(),
    decision_reason = sqlc.narg('decision_reason')::text,
    updated_at      = NOW()
WHERE id = sqlc.arg('id')::uuid
  AND status = 'pending'
RETURNING *;

-- name: RejectTeacherRoleRequest :one
UPDATE teacher_role_requests
SET status          = 'rejected',
    reviewed_by     = sqlc.arg('reviewed_by')::uuid,
    reviewed_at     = NOW(),
    decision_reason = sqlc.arg('decision_reason')::text,
    updated_at      = NOW()
WHERE id = sqlc.arg('id')::uuid
  AND status = 'pending'
RETURNING *;
