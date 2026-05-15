-- name: AddParticipant :one
INSERT INTO retake_participants (retake_id, user_id, kind, debt_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RemoveParticipant :exec
-- Физическое удаление. Если пересдача уже стартовала или есть оценка —
-- сервис должен отказать (проверка в бизнес-слое, не в БД).
DELETE FROM retake_participants
WHERE retake_id = $1
  AND user_id = $2;

-- name: ListParticipantsForRetake :many
-- Все участники пересдачи: студенты и преподаватели/комиссия вместе.
-- На handler-уровне разделяются по полю kind.
SELECT *
FROM retake_participants
WHERE retake_id = $1
ORDER BY kind ASC, created_at ASC;

-- name: ListStudentParticipantsForRetake :many
-- Только студенты пересдачи. Используется при выставлении оценок.
SELECT rp.*, u.email, u.first_name, u.last_name, u.middle_name, u.group_name
FROM retake_participants rp
JOIN users u ON u.id = rp.user_id
WHERE rp.retake_id = $1
  AND rp.kind = 'student'
ORDER BY u.last_name ASC, u.first_name ASC;

-- name: ListTeacherParticipantsForRetake :many
SELECT *
FROM retake_participants
WHERE retake_id = $1
  AND kind IN ('teacher', 'commission_member');

-- name: CountTeachersInRetake :one
-- Проверка инварианта "commission требует ≥ 3 преподавателей" перед
-- старшим переводом пересдачи в активный статус.
SELECT COUNT(*)
FROM retake_participants
WHERE retake_id = $1
  AND kind IN ('teacher', 'commission_member');

-- name: GradeStudentParticipant :one
-- Выставление оценки участнику-студенту. Сервис вызывает это
-- в той же транзакции, что и GradeDebt — чтобы retake_participants
-- и debts остались согласованы.
UPDATE retake_participants
SET grade     = $2,
    graded_at = NOW(),
    graded_by = $3
WHERE id = $1
  AND kind = 'student'
  AND grade IS NULL
RETURNING *;

-- name: GetParticipant :one
SELECT *
FROM retake_participants
WHERE id = $1;

-- name: GetParticipantByRetakeAndUser :one
SELECT *
FROM retake_participants
WHERE retake_id = $1
  AND user_id = $2;
