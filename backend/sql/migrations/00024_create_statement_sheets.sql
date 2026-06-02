-- Ведомость пересдачи (statement sheet) — двухэтапная фиксация оценок.
--
-- Раньше оценка ставилась одноэтапно и НЕОБРАТИМО: преподаватель жал
-- оценку → долг сразу graded + write-back в эмулятор. Ошибся — отмены нет.
--
-- Теперь:
--   1. Сохранить (черновик) — оценки пишутся в retake_participants.grade,
--      долг остаётся open, эмулятор не трогаем. Можно перезаписывать.
--   2. Закрыть ведомость — фиксация: все долги → graded + write-back.
--   3. Открыть ведомость — ТОЛЬКО декан: closed → open, долги → open.
--
-- Статус ведомости (open/closed) — источник правды "зафиксирована ли".
-- Черновики оценок живут в retake_participants (там уже есть grade и
-- история по каждому участнику). Запись sheet создаётся лениво при
-- первом обращении к ведомости (lazy-create через ON CONFLICT).
--
-- reopened_* хранит ПОСЛЕДНИЙ reopen (перезаписывается). Полная история
-- открытий/закрытий — в audit_log (как у retake_requests).
--
-- CHECK-constraints дублируют то, что валидирует сервис, для защиты от
-- прямой записи в БД (миграции/seed/manual fix).

-- +goose Up
CREATE TABLE IF NOT EXISTS statement_sheets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retake_id   UUID NOT NULL REFERENCES retakes(id) ON DELETE CASCADE,
    status      VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    closed_at   TIMESTAMP WITH TIME ZONE,
    closed_by   UUID REFERENCES users(id),
    reopened_at TIMESTAMP WITH TIME ZONE,
    reopened_by UUID REFERENCES users(id),
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE,

    -- closed ⇒ closed_at/closed_by заполнены; open ⇒ оба NULL.
    CONSTRAINT statement_sheets_close_consistency CHECK (
        (status = 'closed' AND closed_at IS NOT NULL AND closed_by IS NOT NULL)
        OR (status = 'open' AND closed_at IS NULL AND closed_by IS NULL)
    ),
    -- reopened_at/reopened_by парные: оба NULL или оба заполнены.
    CONSTRAINT statement_sheets_reopen_consistency CHECK (
        (reopened_at IS NULL AND reopened_by IS NULL)
        OR (reopened_at IS NOT NULL AND reopened_by IS NOT NULL)
    )
);

-- 1:1 с пересдачей + race-safe lazy-create через ON CONFLICT (retake_id).
CREATE UNIQUE INDEX IF NOT EXISTS idx_statement_sheets_retake
    ON statement_sheets (retake_id);

-- +goose Down
DROP TABLE IF EXISTS statement_sheets;
