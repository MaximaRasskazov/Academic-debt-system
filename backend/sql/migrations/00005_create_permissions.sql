-- Разрешения RBAC.
--
-- slug — стабильный идентификатор для проверки в коде (через константы
-- в internal/service/rbac). Имеет формат "<entity>.<action>" или
-- "<entity>.<action>.<scope>", например "debts.view.own".
--
-- is_system=TRUE для базового набора, выдаваемого сидером. Прикладной
-- код должен опираться именно на системные slug'и, кастомные могут
-- удаляться через API.

-- +goose Up
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),
    updated_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    deleted_by UUID REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_permissions_name_lower ON permissions (LOWER(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_permissions_slug_lower ON permissions (LOWER(slug)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_permissions_created_by ON permissions (created_by);
CREATE INDEX IF NOT EXISTS idx_permissions_deleted_at ON permissions (deleted_at);

-- +goose Down
DROP TABLE IF EXISTS permissions;
