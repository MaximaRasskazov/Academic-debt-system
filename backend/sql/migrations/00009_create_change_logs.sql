-- Полные срезы сущностей до/после мутации — для истории и undo.
--
-- По ТЗ системы нужно "хранить историю изменения статусов", поэтому в
-- before/after пишутся ПОЛНЫЕ поля сущности — это даёт и читаемую
-- историю в UI, и техническую возможность откатить запись.
--
-- Семантика action:
--   created           — before={}, after=поля_новой_записи
--   updated           — before=старые_поля, after=новые_поля
--   soft_deleted      — before=поля, after={}
--   hard_deleted      — before=поля, after={}
--   restored          — before=поля_на_момент_удаления, after=восстановленные
--   restored_from_log — before=текущие, after=применённые_из_лога (undo)
--
-- entity_id хранится как VARCHAR(255), потому что в проекте используются
-- UUID, но change_logs универсален и теоретически может ссылаться на
-- сущности с другими типами ключей.

-- +goose Up
CREATE TABLE IF NOT EXISTS change_logs (
    id BIGSERIAL PRIMARY KEY,
    entity_type VARCHAR(64) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    action VARCHAR(32) NOT NULL,
    before JSONB NOT NULL DEFAULT '{}'::jsonb,
    after JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT
);

-- Индексы под основные сценарии чтения:
--   история одной сущности: WHERE entity_type=? AND entity_id=? ORDER BY created_at DESC
--   сводка по типу:         WHERE entity_type=?
--   выборка по автору:      WHERE created_by=?
CREATE INDEX IF NOT EXISTS idx_change_logs_entity ON change_logs (entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_change_logs_entity_type ON change_logs (entity_type);
CREATE INDEX IF NOT EXISTS idx_change_logs_created_by ON change_logs (created_by);
CREATE INDEX IF NOT EXISTS idx_change_logs_created_at ON change_logs (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS change_logs;
