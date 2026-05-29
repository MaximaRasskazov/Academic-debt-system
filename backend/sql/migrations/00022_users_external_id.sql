-- Добавляет external_id к users для idempotent-синхронизации с эмулятором.
-- Поле заполняется SyncService при upsert аккаунтов; ручные пользователи
-- (созданные через регистрацию) оставляют его NULL.

-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS external_id VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external_id
    ON users (external_id) WHERE external_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_external_id;
ALTER TABLE users DROP COLUMN IF EXISTS external_id;
