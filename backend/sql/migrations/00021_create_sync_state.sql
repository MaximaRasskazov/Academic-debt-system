-- Состояние синхронизации с внешним эмулятором.
--
-- Хранит метку времени последнего успешного прогона SyncService.
-- При старте нового цикла сервис читает last_synced_at и передаёт
-- его как параметр ?since= в GET /api/v1/changes эмулятора — получая
-- только изменения, произошедшие после предыдущего прогона.
--
-- Таблица содержит ровно одну строку (key = 'emulator').
-- Отдельная запись на ключ позволит в будущем добавить второй
-- источник без изменения схемы.

-- +goose Up
CREATE TABLE IF NOT EXISTS sync_state (
    key            VARCHAR(64) PRIMARY KEY,
    last_synced_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT '1970-01-01T00:00:00Z'
);

INSERT INTO sync_state (key) VALUES ('emulator') ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS sync_state;
