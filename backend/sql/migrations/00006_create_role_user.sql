-- Связка пользователь ↔ роль (многие-ко-многим).
--
-- Soft-delete через deleted_at позволяет восстанавливать привязку
-- и сохранять историю в change_logs.

-- +goose Up
CREATE TABLE IF NOT EXISTS role_user (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),
    deleted_at TIMESTAMP WITH TIME ZONE,
    deleted_by UUID REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_role_user_unique ON role_user (user_id, role_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_role_user_user_id ON role_user (user_id);
CREATE INDEX IF NOT EXISTS idx_role_user_role_id ON role_user (role_id);
CREATE INDEX IF NOT EXISTS idx_role_user_deleted_at ON role_user (deleted_at);

-- +goose Down
DROP TABLE IF EXISTS role_user;
