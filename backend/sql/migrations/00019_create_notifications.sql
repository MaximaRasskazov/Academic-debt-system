-- Уведомления для real-time-канала и истории.
--
-- ТЗ требует поддержки работы в реальном времени и email-уведомлений
-- о пересдачах. Эта таблица — общий store уведомлений: они и пушатся
-- в WebSocket по подписке user_id, и из них же фронт может подгрузить
-- историю при первой загрузке (а не только новые).
--
-- kind — короткий слаг события для UI и фильтров:
--   retake_scheduled
--   retake_updated
--   retake_cancelled
--   retake_grade_received
--   teacher_request_approved / rejected
--   retake_change_request_approved / rejected
-- (точный список фиксируется в коде на этапе уведомлений.)
--
-- payload — произвольный JSONB. Конкретные поля зависят от kind.
-- Например, для retake_scheduled: {retake_id, discipline, scheduled_at,
-- location}.
--
-- read_at = NULL означает "ещё не просмотрено", идёт в счётчик badge на
-- иконке колокольчика. Маркируется POST /api/notifications/:id/read.

-- +goose Up
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Горячий путь: непрочитанные пользователя для badge и списка.
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id, created_at DESC) WHERE read_at IS NULL;
-- Общая история пользователя (для страницы "все уведомления").
CREATE INDEX IF NOT EXISTS idx_notifications_user
    ON notifications (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_kind ON notifications (kind);

-- +goose Down
DROP TABLE IF EXISTS notifications;
