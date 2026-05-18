-- Пересдачи.
--
-- Поля по ТЗ: дисциплина, тип, корпус+аудитория, дата+время,
-- продолжительность, статус. Список студентов и преподавателей —
-- в отдельной таблице retake_participants (см. 00016).
--
-- kind:
--   regular    — обычная (1 преподаватель достаточно);
--   commission — с комиссией (требуется минимум 3 преподавателя).
-- Минимум 3 проверяется на сервисном слое при создании/обновлении.
-- В БД храним min_teachers как явное значение — это позволяет:
--   1) при kind=commission гарантировать в CHECK, что min_teachers >= 3;
--   2) гибко поднять минимум до 4 в будущем без миграции схемы.
--
-- Статусы:
--   scheduled   — назначена, ожидает проведения;
--   in_progress — началась (scheduled_at прошёл, completed_at ещё нет);
--   completed   — завершена (фактически — completed_at заполнен);
--   cancelled   — отменена деканатом.
-- Переход scheduled→in_progress→completed выполняет шедулер
-- (этап 11 плана). completed_at — фактическое время завершения
-- (может быть позже scheduled_at + duration на пару минут из-за
-- интервала тика шедулера).

-- +goose Up
CREATE TABLE IF NOT EXISTS retakes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discipline_id UUID NOT NULL REFERENCES disciplines(id) ON DELETE RESTRICT,
    kind VARCHAR(20) NOT NULL DEFAULT 'regular' CHECK (kind IN ('regular', 'commission')),
    min_teachers INTEGER NOT NULL DEFAULT 1,
    building VARCHAR(50) NOT NULL,
    room VARCHAR(50) NOT NULL,
    scheduled_at TIMESTAMP WITH TIME ZONE NOT NULL,
    duration_minutes INTEGER NOT NULL CHECK (duration_minutes > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'in_progress', 'completed', 'cancelled')),
    completed_at TIMESTAMP WITH TIME ZONE,
    created_by UUID NOT NULL REFERENCES users(id),
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    deleted_by UUID REFERENCES users(id),

    -- Для пересдачи с комиссией min_teachers как минимум 3 (требование ТЗ).
    CONSTRAINT retakes_commission_min_teachers CHECK (
        kind = 'regular' OR (kind = 'commission' AND min_teachers >= 3)
    ),
    -- completed_at заполнен только когда статус completed.
    CONSTRAINT retakes_completed_consistency CHECK (
        (status = 'completed' AND completed_at IS NOT NULL)
        OR (status <> 'completed' AND completed_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_retakes_discipline ON retakes (discipline_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_retakes_status_scheduled_at ON retakes (status, scheduled_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_retakes_scheduled_at ON retakes (scheduled_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_retakes_completed_at ON retakes (completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_retakes_created_by ON retakes (created_by);

-- +goose Down
DROP TABLE IF EXISTS retakes;
