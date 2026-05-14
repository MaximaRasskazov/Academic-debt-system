-- Связка роль ↔ разрешение (многие-ко-многим).
--
-- При проверке доступа: user → role_user → role → permission_role →
-- permission.slug. Soft-delete позволяет временно отзывать разрешение
-- у роли без потери истории.

-- +goose Up
CREATE TABLE IF NOT EXISTS permission_role (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),
    deleted_at TIMESTAMP WITH TIME ZONE,
    deleted_by UUID REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_permission_role_unique ON permission_role (role_id, permission_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_permission_role_role_id ON permission_role (role_id);
CREATE INDEX IF NOT EXISTS idx_permission_role_permission_id ON permission_role (permission_id);
CREATE INDEX IF NOT EXISTS idx_permission_role_deleted_at ON permission_role (deleted_at);

-- +goose Down
DROP TABLE IF EXISTS permission_role;
