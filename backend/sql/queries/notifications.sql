-- name: CreateNotification :one
INSERT INTO notifications (user_id, kind, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetNotificationByID :one
SELECT *
FROM notifications
WHERE id = $1;

-- name: ListNotificationsForUser :many
-- Лента пользователя: вся история уведомлений с пагинацией.
SELECT *
FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUnreadNotificationsForUser :many
-- Для подписки WebSocket-клиента на reconnect: догнать пропущенное.
SELECT *
FROM notifications
WHERE user_id = $1
  AND read_at IS NULL
ORDER BY created_at DESC;

-- name: CountUnreadForUser :one
-- Badge-counter в шапке UI.
SELECT COUNT(*)
FROM notifications
WHERE user_id = $1
  AND read_at IS NULL;

-- name: MarkNotificationRead :exec
-- Идемпотентно: повторный вызов не перезаписывает read_at.
UPDATE notifications
SET read_at = NOW()
WHERE id = $1
  AND user_id = $2
  AND read_at IS NULL;

-- name: MarkAllNotificationsReadForUser :execrows
UPDATE notifications
SET read_at = NOW()
WHERE user_id = $1
  AND read_at IS NULL;
