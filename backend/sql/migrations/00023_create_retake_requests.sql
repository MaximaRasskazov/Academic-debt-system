-- Заявки преподавателей на создание новой пересдачи.
--
-- Семантически это "preflight" над таблицей retakes: преподаватель
-- описывает желаемую пересдачу (дата, место, состав), деканат смотрит
-- и одобряет/отклоняет. При одобрении сервис атомарно создаёт пересдачу
-- и привязывает участников; payload теряется после approve — для
-- истории достаточно audit_log + созданной retake-записи.
--
-- Отличие от retake_change_requests:
--   retake_change_requests — преподаватель просит ИЗМЕНИТЬ существующую
--   retake_requests        — преподаватель просит СОЗДАТЬ новую
--
-- payload — JSONB с полями discipline_id, kind, scheduled_at,
-- duration_minutes, building, room, notes, reason плюс массивы
-- student_debt_ids и teacher_ids. JSONB здесь оправдан: payload
-- используется только как заявка-сообщение для деканата, а не как
-- источник истины — реальные данные после approve лежат в retakes
-- и retake_participants. Делать 8 колонок + 2 join-таблицы ради заявки,
-- которая живёт один раз — overengineering.
--
-- decision_reason обязателен при rejected — требование ТЗ "при отказе
-- указать причину". CHECK дублирует то, что валидирует сервис, для
-- защиты от прямой записи в БД (миграции/seed/manual fix).

-- +goose Up
CREATE TABLE IF NOT EXISTS retake_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requested_by UUID NOT NULL REFERENCES users(id),
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMP WITH TIME ZONE,
    decision_reason TEXT,
    created_retake_id UUID REFERENCES retakes(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE,

    -- При отклонении должна быть причина.
    CONSTRAINT retake_requests_rejection_reason CHECK (
        status <> 'rejected' OR (decision_reason IS NOT NULL AND length(trim(decision_reason)) > 0)
    ),
    -- pending → reviewed_*-поля пустые; approved/rejected → заполнены.
    CONSTRAINT retake_requests_review_consistency CHECK (
        (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL)
        OR (status IN ('approved', 'rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    ),
    -- approved обязан указывать на созданную пересдачу.
    -- (Гарантия: после одобрения мы реально создали retake в той же tx.)
    CONSTRAINT retake_requests_approved_has_retake CHECK (
        status <> 'approved' OR created_retake_id IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS idx_retake_requests_status ON retake_requests (status);
CREATE INDEX IF NOT EXISTS idx_retake_requests_requested_by ON retake_requests (requested_by);
CREATE INDEX IF NOT EXISTS idx_retake_requests_created_at ON retake_requests (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS retake_requests;
