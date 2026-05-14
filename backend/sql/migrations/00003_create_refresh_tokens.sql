-- Refresh-токены: одноразовые, связаны 1:1 с конкретным access-токеном.
--
-- Хранится только SHA-256 хеш. Логика one-use реализована флагом
-- is_used. Попытка повторного использования помеченного токена должна
-- триггерить отзыв всех токенов пользователя (защита от replay при
-- краже refresh — реализуется на уровне сервиса).

-- +goose Up
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    access_token_id UUID NOT NULL REFERENCES access_tokens(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_access_token_id ON refresh_tokens (access_token_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
