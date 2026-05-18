-- name: AttachStudentToDiscipline :one
INSERT INTO student_disciplines (student_id, discipline_id, academic_year, semester, source)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DetachStudentFromDiscipline :exec
UPDATE student_disciplines
SET deleted_at = NOW()
WHERE student_id = $1
  AND discipline_id = $2
  AND deleted_at IS NULL;

-- name: ListDisciplinesForStudent :many
-- Какие дисциплины изучает студент. Используется в личном кабинете
-- и при создании долга (валидируем, что студент учится на дисциплине).
SELECT d.*
FROM disciplines d
JOIN student_disciplines sd ON sd.discipline_id = d.id
WHERE sd.student_id = $1
  AND sd.deleted_at IS NULL
  AND d.deleted_at IS NULL
ORDER BY d.name ASC;

-- name: ListStudentsInDiscipline :many
-- Кто на дисциплине учится. Деканат использует для сводных отчётов.
SELECT u.*
FROM users u
JOIN student_disciplines sd ON sd.student_id = u.id
WHERE sd.discipline_id = $1
  AND sd.deleted_at IS NULL
ORDER BY u.last_name ASC, u.first_name ASC;

-- name: IsStudentEnrolled :one
-- Быстрая проверка для сервиса debts: можно ли поставить долг
-- этому студенту по этой дисциплине.
SELECT EXISTS (
    SELECT 1
    FROM student_disciplines
    WHERE student_id = $1
      AND discipline_id = $2
      AND deleted_at IS NULL
) AS enrolled;
