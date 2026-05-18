-- Кто из преподавателей какие дисциплины ведёт.
--
-- Используется RBAC-фильтром "преподаватель видит долги по своим
-- дисциплинам" (permission debts.view.by_discipline) — без этой
-- таблицы фильтр бессмыслен.
--
-- created_by — кто назначил преподавателя на дисциплину (обычно
-- деканат или админ). Soft-delete через deleted_at — снятие с
-- дисциплины не теряет историю.

-- +goose Up
CREATE TABLE IF NOT EXISTS teacher_disciplines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    discipline_id UUID NOT NULL REFERENCES disciplines(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),
    deleted_at TIMESTAMP WITH TIME ZONE,
    deleted_by UUID REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_teacher_disciplines_unique
    ON teacher_disciplines (teacher_id, discipline_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_teacher_disciplines_teacher ON teacher_disciplines (teacher_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_teacher_disciplines_discipline ON teacher_disciplines (discipline_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS teacher_disciplines;
