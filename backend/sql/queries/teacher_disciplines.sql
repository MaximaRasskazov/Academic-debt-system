-- name: AttachTeacherToDiscipline :one
INSERT INTO teacher_disciplines (teacher_id, discipline_id, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DetachTeacherFromDiscipline :exec
UPDATE teacher_disciplines
SET deleted_at = NOW(),
    deleted_by = $3
WHERE teacher_id = $1
  AND discipline_id = $2
  AND deleted_at IS NULL;

-- name: ListDisciplinesForTeacher :many
-- Дисциплины конкретного преподавателя. Используется в его кабинете
-- и для построения фильтра debts.view.by_discipline.
SELECT d.*
FROM disciplines d
JOIN teacher_disciplines td ON td.discipline_id = d.id
WHERE td.teacher_id = $1
  AND td.deleted_at IS NULL
  AND d.deleted_at IS NULL
ORDER BY d.name ASC;

-- name: ListTeachersForDiscipline :many
SELECT u.*
FROM users u
JOIN teacher_disciplines td ON td.teacher_id = u.id
WHERE td.discipline_id = $1
  AND td.deleted_at IS NULL
ORDER BY u.last_name ASC, u.first_name ASC;

-- name: IsTeacherOfDiscipline :one
-- RBAC-фильтр: преподаватель имеет debts.view.by_discipline только
-- для своих дисциплин. Сервис вызывает это перед выдачей списка долгов.
SELECT EXISTS (
    SELECT 1
    FROM teacher_disciplines
    WHERE teacher_id = $1
      AND discipline_id = $2
      AND deleted_at IS NULL
) AS is_teacher;
