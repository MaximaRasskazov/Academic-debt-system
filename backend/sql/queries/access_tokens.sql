-- name: CreateAccessToken :one
INSERT INTO access_tokens (user_id, token_hash, expires_at, ip_address)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAccessTokenByHash :one
-- Используется в auth-middleware для проверки выданного access-токена.
-- Возвращает запись только если она активна (не отозвана и не истекла).
SELECT *
FROM access_tokens
WHERE token_hash = $1
  AND is_revoked = FALSE
  AND expires_at > NOW();

-- name: GetAccessTokenByID :one
SELECT *
FROM access_tokens
WHERE id = $1;

-- name: TouchAccessToken :exec
-- Обновляет last_used_at при успешной проверке токена в middleware.
UPDATE access_tokens
SET last_used_at = NOW()
WHERE id = $1;

-- name: RevokeAccessToken :exec
UPDATE access_tokens
SET is_revoked = TRUE
WHERE id = $1;

-- name: RevokeAllAccessTokensForUser :exec
-- Используется при logout-all и при детекте повторного использования
-- refresh-токена (защита от replay).
UPDATE access_tokens
SET is_revoked = TRUE
WHERE user_id = $1
  AND is_revoked = FALSE;

-- name: DeleteExpiredAccessTokens :execrows
-- Периодическая чистка таблицы фоновой задачей.
DELETE FROM access_tokens
WHERE expires_at < NOW() - INTERVAL '7 days';
