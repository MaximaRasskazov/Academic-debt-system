-- Журнал аудита событий: фиксирует ЧТО произошло, не дампит сами данные.
--
-- Используется для security-чувствительных действий: вход в систему,
-- смена пароля, выдача/отзыв роли, попытка privilege escalation,
-- создание долга, отмена пересдачи и т.п.
--
-- Полные дампы полей сущности храним отдельно — в change_logs (см.
-- следующую миграцию).

-- +goose Up
CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID REFERENCES users(id),
    action VARCHAR(64) NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id VARCHAR(255),
    details JSONB,
    ip_address VARCHAR(45),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_actor_id ON audit_log (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log (action);
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON audit_log (target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log (created_at);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
