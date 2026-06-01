-- name: CreateStatementSheet :one
-- Идемпотентный lazy-create ведомости. При гонке первого обращения
-- (два запроса одновременно) второй INSERT ничего не делает из-за
-- UNIQUE на retake_id — сервис затем читает строку через
-- GetStatementSheetByRetake. RETURNING вернёт строку только когда
-- вставка реально произошла; при конфликте — пусто (ErrNoRows).
INSERT INTO statement_sheets (retake_id)
VALUES ($1)
ON CONFLICT (retake_id) DO NOTHING
RETURNING *;

-- name: GetStatementSheetByRetake :one
SELECT *
FROM statement_sheets
WHERE retake_id = $1;

-- name: CloseStatementSheet :one
-- open → closed. WHERE status='open' отсекает повторное закрытие/гонку
-- (вернёт ErrNoRows → сервис трактует как ErrSheetClosed).
UPDATE statement_sheets
SET status     = 'closed',
    closed_at  = NOW(),
    closed_by  = $2,
    updated_at = NOW()
WHERE retake_id = $1
  AND status = 'open'
RETURNING *;

-- name: ReopenStatementSheet :one
-- closed → open (декан). Сбрасываем closed_*, фиксируем reopened_*.
-- WHERE status='closed' отсекает повторное открытие/гонку.
UPDATE statement_sheets
SET status      = 'open',
    closed_at   = NULL,
    closed_by   = NULL,
    reopened_at = NOW(),
    reopened_by = $2,
    updated_at  = NOW()
WHERE retake_id = $1
  AND status = 'closed'
RETURNING *;
