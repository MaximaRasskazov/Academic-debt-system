-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (access_token_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByHash :one
-- Возвращает запись по хешу. Сервис auth дополнительно проверяет is_used
-- и is_revoked — если is_used=TRUE при попытке refresh, это replay
-- и сервис должен отозвать все токены пользователя.
SELECT *
FROM refresh_tokens
WHERE token_hash = $1;

-- name: GetRefreshTokenByHashForUpdate :one
-- То же, что GetRefreshTokenByHash, но с блокировкой строки FOR UPDATE.
-- Используется внутри RunInTx в token.Rotate, чтобы две параллельные
-- попытки обмена одного refresh не прошли проверку is_used=FALSE
-- одновременно — второй вызов будет ждать коммита первого.
SELECT *
FROM refresh_tokens
WHERE token_hash = $1
FOR UPDATE;

-- name: MarkRefreshTokenUsed :exec
UPDATE refresh_tokens
SET is_used = TRUE
WHERE id = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET is_revoked = TRUE
WHERE id = $1;

-- name: RevokeAllRefreshTokensForAccessToken :exec
-- Отзыв всех refresh-токенов, связанных с конкретным access-токеном.
UPDATE refresh_tokens
SET is_revoked = TRUE
WHERE access_token_id = $1
  AND is_revoked = FALSE;

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens
WHERE expires_at < NOW() - INTERVAL '7 days';
