-- Кто из студентов на каких дисциплинах учится.
--
-- Нужна для:
--   - синхронизации с внешней системой (внешка отдаёт списки
--     "студент → его дисциплины");
--   - сводных таблиц деканата ("по дисциплине X должников Y из Z");
--   - валидации создания долга преподавателем (студент должен учиться
--     на этой дисциплине — иначе откуда взяться долгу).
--
-- academic_year / semester опциональные, заполняются из синхронизации;
-- их можно использовать в фильтрах отчётов "за весенний семестр 2026".

-- +goose Up
CREATE TABLE IF NOT EXISTS student_disciplines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    discipline_id UUID NOT NULL REFERENCES disciplines(id) ON DELETE CASCADE,
    academic_year VARCHAR(20),
    semester INTEGER CHECK (semester IS NULL OR semester IN (1, 2)),
    source VARCHAR(20) NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'sync')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Один студент — одна активная привязка к дисциплине.
-- Если нужны историчные записи "был на этом семестре" — soft-delete
-- предыдущей и вставка новой.
CREATE UNIQUE INDEX IF NOT EXISTS idx_student_disciplines_unique
    ON student_disciplines (student_id, discipline_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_student_disciplines_student ON student_disciplines (student_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_student_disciplines_discipline ON student_disciplines (discipline_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS student_disciplines;
