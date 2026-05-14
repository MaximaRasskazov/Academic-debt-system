-- Роли RBAC.
--
-- level — числовой уровень роли (admin=1000, dean=700, teacher=300,
-- student=100). Используется для запрета "повышения над собой": роль
-- с level=X не может выдать другому пользователю роль с level >= X.
--
-- is_system=TRUE защищает системные роли (student/teacher/dean/admin)
-- от удаления и переименования через API.

-- +goose Up
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    level INTEGER NOT NULL DEFAULT 0,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),
    updated_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    deleted_by UUID REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_name_lower ON roles (LOWER(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_slug_lower ON roles (LOWER(slug)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_roles_created_by ON roles (created_by);
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles (deleted_at);

-- +goose Down
DROP TABLE IF EXISTS roles;
