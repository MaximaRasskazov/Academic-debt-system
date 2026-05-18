-- Заявки на получение роли "Преподаватель".
--
-- По ТЗ: при регистрации пользователь получает роль "Студент".
-- Чтобы стать преподавателем — оставляет заявку, деканат рассматривает.
--
-- UNIQUE-индекс idx_teacher_role_requests_unique_pending запрещает
-- держать два активных pending-запроса на одного пользователя —
-- спамить заявками не получится. Чтобы подать новую, нужно дождаться
-- рассмотрения старой (или отозвать её — отдельный сценарий, пока
-- не реализуется).
--
-- При одобрении заявки сервисный слой в одной транзакции:
--   1) проставляет status='approved', reviewed_by/reviewed_at;
--   2) добавляет роль teacher через role_user.
-- При отклонении decision_reason обязателен.

-- +goose Up
CREATE TABLE IF NOT EXISTS teacher_role_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requested_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMP WITH TIME ZONE,
    decision_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT teacher_role_requests_rejection_reason CHECK (
        status <> 'rejected' OR (decision_reason IS NOT NULL AND length(trim(decision_reason)) > 0)
    ),
    CONSTRAINT teacher_role_requests_review_consistency CHECK (
        (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL)
        OR (status IN ('approved', 'rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_teacher_role_requests_requested_by ON teacher_role_requests (requested_by);
CREATE INDEX IF NOT EXISTS idx_teacher_role_requests_status ON teacher_role_requests (status);
CREATE INDEX IF NOT EXISTS idx_teacher_role_requests_created_at ON teacher_role_requests (created_at DESC);
-- Один pending-запрос на пользователя.
CREATE UNIQUE INDEX IF NOT EXISTS idx_teacher_role_requests_unique_pending
    ON teacher_role_requests (requested_by) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS teacher_role_requests;
