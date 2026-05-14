-- Таблица пользователей системы.
--
-- Один пользователь — одна учётная запись. Роли подключаются через
-- role_user (см. миграцию 00006). Логин выполняется по email — username
-- как отдельного поля нет.
--
-- Академические поля:
--   first_name / last_name — обязательны для отображения в интерфейсах
--                            (списки студентов, ведомости, отчёты);
--   middle_name            — отчество, опционально (иностранные студенты
--                            могут не иметь);
--   birthday               — опционально, используется в верификации
--                            возраста при некоторых операциях;
--   group_name             — учебная группа, заполнено только для
--                            пользователей с ролью student.

-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    middle_name VARCHAR(100),
    birthday DATE,
    group_name VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));
CREATE INDEX IF NOT EXISTS idx_users_group_name ON users (group_name) WHERE group_name IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS users;