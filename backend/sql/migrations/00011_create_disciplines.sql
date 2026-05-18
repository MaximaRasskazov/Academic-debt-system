-- Справочник дисциплин.
--
-- Управляется деканатом/админом через CRUD-эндпоинты. Может частично
-- наполняться через синхронизацию с внутренней системой успеваемости —
-- для этого хранится source ('manual' / 'sync') и external_id (для
-- idempotent-апсерта при повторной синхронизации).
--
-- code — короткий идентификатор для UI (например "MATH-101"), name —
-- полное название. Оба уникальны (case-insensitive) среди активных.

-- +goose Up
CREATE TABLE IF NOT EXISTS disciplines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) NOT NULL,
    description TEXT,
    external_id VARCHAR(255),
    source VARCHAR(20) NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'sync')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),
    updated_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    deleted_by UUID REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_disciplines_code_lower ON disciplines (LOWER(code)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_disciplines_name_lower ON disciplines (LOWER(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_disciplines_external_id ON disciplines (external_id) WHERE external_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_disciplines_deleted_at ON disciplines (deleted_at);

-- +goose Down
DROP TABLE IF EXISTS disciplines;
