-- Академические долги студентов.
--
-- По ТЗ долг связывает: студент + дисциплина + преподаватель, который
-- его поставил. Статусы:
--   open      — долг открыт, ожидает пересдачи или ручного закрытия;
--   graded    — закрыт оценкой (final_grade проставлен);
--   cancelled — отменён деканатом (например, ошибка преподавателя).
--
-- Преподаватель меняет долг на оценку через переход статуса open→graded,
-- при этом ТЗ требует фиксировать дату/время — это поле graded_at.
-- Дополнительно graded_by, чтобы видеть, какой преподаватель закрыл
-- (может отличаться от issued_by, если шла пересдача с другим препом).
--
-- external_id + source — для idempotent-синхронизации с внешней
-- системой успеваемости (повторный запуск синхронизации не создаёт
-- дубли благодаря UNIQUE-индексу idx_debts_external_id).
--
-- final_grade ограничен 2..5 (российская шкала); расширить можно
-- через миграцию правки CHECK.
--
-- Soft-delete через deleted_at: разрешение debts.delete (есть в RBAC)
-- помечает запись удалённой, не физически.
--
-- UNIQUE idx_debts_unique_open гарантирует: у одного студента не может
-- быть одновременно двух открытых долгов по одной дисциплине. Если
-- старый закрыт оценкой или отменён — новый можно ставить.

-- +goose Up
CREATE TABLE IF NOT EXISTS debts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    discipline_id UUID NOT NULL REFERENCES disciplines(id) ON DELETE RESTRICT,
    issued_by UUID NOT NULL REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'graded', 'cancelled')),
    final_grade INTEGER CHECK (final_grade IS NULL OR final_grade BETWEEN 2 AND 5),
    graded_at TIMESTAMP WITH TIME ZONE,
    graded_by UUID REFERENCES users(id),
    external_id VARCHAR(255),
    source VARCHAR(20) NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'sync')),
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    deleted_by UUID REFERENCES users(id),

    -- Согласованность графа закрытия: для graded требуется и оценка,
    -- и кем/когда поставлена. Для остальных статусов эти поля NULL.
    CONSTRAINT debts_grade_consistency CHECK (
        (status = 'graded' AND final_grade IS NOT NULL AND graded_at IS NOT NULL AND graded_by IS NOT NULL)
        OR (status <> 'graded' AND final_grade IS NULL AND graded_at IS NULL AND graded_by IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_debts_student ON debts (student_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_debts_discipline ON debts (discipline_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_debts_status ON debts (status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_debts_issued_by ON debts (issued_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_debts_external_id ON debts (external_id) WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_debts_unique_open
    ON debts (student_id, discipline_id) WHERE deleted_at IS NULL AND status = 'open';

-- +goose Down
DROP TABLE IF EXISTS debts;
