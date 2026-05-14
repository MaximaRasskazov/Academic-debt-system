-- Хранилище хешей выданных access-токенов.
--
-- Сам JWT клиенту отдаётся в plaintext, но в БД хранится только SHA-256
-- хеш — это позволяет отзывать токены до истечения срока (через
-- is_revoked) и трекать "последнее использование" / IP без раскрытия
-- секретов в случае утечки БД.

-- +goose Up
CREATE TABLE IF NOT EXISTS access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE,
    ip_address VARCHAR(45)
);

CREATE INDEX IF NOT EXISTS idx_access_tokens_token_hash ON access_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_access_tokens_active ON access_tokens (user_id, is_revoked, expires_at);
CREATE INDEX IF NOT EXISTS idx_access_tokens_user_id ON access_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS access_tokens;
