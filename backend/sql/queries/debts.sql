-- name: CreateDebt :one
INSERT INTO debts (student_id, discipline_id, issued_by, external_id, source, notes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetDebtByID :one
SELECT *
FROM debts
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetDebtByExternalID :one
-- Используется синхронизатором: повторный pull из внешней системы
-- не создаёт дубль, а обновляет существующий долг.
SELECT *
FROM debts
WHERE external_id = $1
  AND deleted_at IS NULL;

-- name: ListDebtsForStudent :many
-- Студент видит свои долги (permission debts.view.own).
SELECT *
FROM debts
WHERE student_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ListDebtsByDiscipline :many
-- Преподаватель: должники по конкретной дисциплине. Сервис
-- предварительно проверяет teacher_disciplines.IsTeacherOfDiscipline.
SELECT *
FROM debts
WHERE discipline_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListDebtsByDisciplines :many
-- Преподаватель: должники по всем своим дисциплинам разом.
-- Передаётся массив id, который сервис собирает из ListDisciplines-
-- ForTeacher. Это эффективнее, чем N+1 запрос.
SELECT *
FROM debts
WHERE discipline_id = ANY(sqlc.arg('discipline_ids')::uuid[])
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListAllDebts :many
-- Деканат: общий список (debts.view.all) с пагинацией.
SELECT *
FROM debts
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountDebts :one
SELECT COUNT(*) FROM debts WHERE deleted_at IS NULL;

-- name: CountOpenDebtsForStudent :one
SELECT COUNT(*)
FROM debts
WHERE student_id = $1
  AND status = 'open'
  AND deleted_at IS NULL;

-- name: GradeDebt :one
-- Перевод долга в graded со всеми требуемыми CHECK-полями.
-- Сервис вызывает это в одной транзакции с обновлением
-- retake_participants.grade на соответствующей пересдаче.
UPDATE debts
SET status      = 'graded',
    final_grade = $2,
    graded_at   = NOW(),
    graded_by   = $3,
    updated_at  = NOW()
WHERE id = $1
  AND status = 'open'
  AND deleted_at IS NULL
RETURNING *;

-- name: CancelDebt :exec
-- Деканат отменяет ошибочно поставленный долг.
UPDATE debts
SET status     = 'cancelled',
    updated_at = NOW()
WHERE id = $1
  AND status = 'open'
  AND deleted_at IS NULL;

-- name: SoftDeleteDebt :exec
UPDATE debts
SET deleted_at = NOW(),
    deleted_by = $2,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: SummaryDebtsByDiscipline :many
-- Сводная для деканата: сколько открытых долгов и сколько
-- закрытых оценкой по каждой дисциплине.
SELECT
    d.discipline_id,
    COUNT(*) FILTER (WHERE d.status = 'open')   AS open_count,
    COUNT(*) FILTER (WHERE d.status = 'graded') AS graded_count
FROM debts d
WHERE d.deleted_at IS NULL
GROUP BY d.discipline_id
ORDER BY open_count DESC;
