-- Заявки преподавателей на изменение пересдачи.
--
-- По ТЗ: преподаватель может подать заявку на изменение времени
-- и/или места пересдачи, указав возможные варианты (диапазоном или
-- перечислением). Деканат рассматривает заявку и одобряет/отклоняет.
-- При отклонении обязательна причина (это требование ТЗ).
--
-- requested_changes — JSONB, формат на стороне сервиса. Ожидаемые
-- ключи (валидируются на handler/service-слое):
--   datetime_options : массив ISO-8601 timestamp'ов
--   location_options : массив объектов {building, room}
--   duration_minutes : опционально
-- JSONB удобнее жёсткой схемы из-за разнородности "перечисление" и
-- "диапазон" вариантов из ТЗ.
--
-- decision_reason обязателен при rejected (CHECK), при approved
-- опционален (деканат может комментарием поделиться).

-- +goose Up
CREATE TABLE IF NOT EXISTS retake_change_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retake_id UUID NOT NULL REFERENCES retakes(id) ON DELETE CASCADE,
    requested_by UUID NOT NULL REFERENCES users(id),
    requested_changes JSONB NOT NULL,
    reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMP WITH TIME ZONE,
    decision_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE,

    -- Требование ТЗ: при отклонении должна быть причина.
    CONSTRAINT change_requests_rejection_reason CHECK (
        status <> 'rejected' OR (decision_reason IS NOT NULL AND length(trim(decision_reason)) > 0)
    ),
    -- pending → reviewed_*-поля пустые; approved/rejected → заполнены.
    CONSTRAINT change_requests_review_consistency CHECK (
        (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL)
        OR (status IN ('approved', 'rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_retake_change_requests_retake ON retake_change_requests (retake_id);
CREATE INDEX IF NOT EXISTS idx_retake_change_requests_status ON retake_change_requests (status);
CREATE INDEX IF NOT EXISTS idx_retake_change_requests_requested_by ON retake_change_requests (requested_by);
CREATE INDEX IF NOT EXISTS idx_retake_change_requests_created_at ON retake_change_requests (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS retake_change_requests;
