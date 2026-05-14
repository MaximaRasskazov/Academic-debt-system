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
-- Пагинация: LIMIT $1, OFFSET $2. Сортировка по дате создания убывающая
-- (новые сверху), стабильный tie-break через id.
SELECT *
FROM users
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

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
