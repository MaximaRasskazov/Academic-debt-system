-- name: CreateUser :one
INSERT INTO users (email, password_hash, first_name, last_name, middle_name, birthday, group_name)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
-- Поиск пользователя по email для логина. Сравнение регистронезависимое
-- (соответствует UNIQUE-индексу idx_users_email_lower).
SELECT *
FROM users
WHERE LOWER(email) = LOWER($1);

-- name: ListUsers :many
-- Список пользователей с опциональными фильтрами. Используется для
-- админ-панели и для UI деканата (выбор преподавателя при создании
-- пересдачи, выбор студента для привязки к дисциплине).
--
-- Все фильтры опциональны:
--   role_slug   — если задан, отдаём только пользователей с этой ролью
--                 (JOIN с role_user + roles WHERE r.slug = ...)
--   search      — подстрока (ILIKE) по email, first_name, last_name
--   group_name  — точный матч (для отбора студентов по группе)
--
-- Сортировка по фамилии — так удобнее листать. limit/offset для пагинации.
SELECT DISTINCT u.*
FROM users u
LEFT JOIN role_user ru ON ru.user_id = u.id AND ru.deleted_at IS NULL
LEFT JOIN roles r      ON r.id = ru.role_id  AND r.deleted_at  IS NULL
WHERE
    (sqlc.narg('role_slug')::text IS NULL OR r.slug = sqlc.narg('role_slug')::text)
AND (sqlc.narg('search')::text    IS NULL OR (
        u.email      ILIKE '%' || sqlc.narg('search')::text || '%'
     OR u.first_name ILIKE '%' || sqlc.narg('search')::text || '%'
     OR u.last_name  ILIKE '%' || sqlc.narg('search')::text || '%'
    ))
AND (sqlc.narg('group_name')::text IS NULL OR u.group_name = sqlc.narg('group_name')::text)
ORDER BY u.last_name ASC, u.first_name ASC, u.id ASC
LIMIT sqlc.arg('limit_n')::int OFFSET sqlc.arg('offset_n')::int;

-- name: CountUsers :one
-- Считает с теми же фильтрами что и ListUsers — для total в пагинации.
SELECT COUNT(DISTINCT u.id)
FROM users u
LEFT JOIN role_user ru ON ru.user_id = u.id AND ru.deleted_at IS NULL
LEFT JOIN roles r      ON r.id = ru.role_id  AND r.deleted_at  IS NULL
WHERE
    (sqlc.narg('role_slug')::text IS NULL OR r.slug = sqlc.narg('role_slug')::text)
AND (sqlc.narg('search')::text    IS NULL OR (
        u.email      ILIKE '%' || sqlc.narg('search')::text || '%'
     OR u.first_name ILIKE '%' || sqlc.narg('search')::text || '%'
     OR u.last_name  ILIKE '%' || sqlc.narg('search')::text || '%'
    ))
AND (sqlc.narg('group_name')::text IS NULL OR u.group_name = sqlc.narg('group_name')::text);

-- name: ListUsersByIDs :many
-- Батч-выборка пользователей по списку ID. Используется в report.Service
-- вместо N одиночных GetUserByID.
SELECT *
FROM users
WHERE id = ANY($1::uuid[]);

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserProfile :one
-- Точечное обновление профиля. NULL-значения в параметрах оставляют поле
-- без изменений (COALESCE), что упрощает PATCH-семантику на handler-слое.
UPDATE users
SET first_name  = COALESCE(sqlc.narg('first_name'),  first_name),
    last_name   = COALESCE(sqlc.narg('last_name'),   last_name),
    middle_name = COALESCE(sqlc.narg('middle_name'), middle_name),
    birthday    = COALESCE(sqlc.narg('birthday'),    birthday),
    group_name  = COALESCE(sqlc.narg('group_name'),  group_name),
    updated_at  = NOW()
WHERE id = sqlc.arg('id')
RETURNING *;
