-- Демо-аккаунты преподавателя и студента для локальной разработки и QA.
--
-- Продолжение цепочки dev-сидов (goose-таблица goose_seed_version, см.
-- entrypoint.sh): накатывается при SEED_DEV_ACCOUNTS=true, в prod выключено.
--
-- В норме преподаватели и студенты приходят из эмулятора деканата через
-- sync.Service (см. 00001_seed_dev_accounts.sql). Но чтобы можно было
-- проверять интерфейсы преподавателя и студента БЕЗ эмулятора, держим по
-- одному готовому аккаунту каждой роли.
--
-- Состав:
--   teacher@academic.local — роль teacher (level 300)
--   student@academic.local — роль student (level 100), группа БСБО-01-22
--
-- Пароль у обоих: "password" (bcrypt cost=10) — как и у admin/dean.
-- ID детерминированные, чтобы повторный накат на инициализированной БД
-- через ON CONFLICT (id) DO NOTHING не дублировал данные.

-- +goose Up
-- +goose StatementBegin

INSERT INTO users (id, email, password_hash, first_name, last_name, middle_name, group_name)
VALUES
    ('cccc0000-0000-0000-0000-000000000001', 'teacher@academic.local', '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Пётр', 'Преподавателев', 'Петрович',  NULL),
    ('dddd0000-0000-0000-0000-000000000001', 'student@academic.local', '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Анна', 'Студентова',     'Андреевна', 'БСБО-01-22')
ON CONFLICT (id) DO NOTHING;

-- Привязка ролей. created_by — служебный системный user из 00010_seed_rbac.
INSERT INTO role_user (user_id, role_id, created_by)
SELECT m.user_id, m.role_id, '00000000-0000-0000-0000-000000000000'::uuid
FROM (VALUES
    ('cccc0000-0000-0000-0000-000000000001'::uuid, '33333333-3333-3333-3333-333333333333'::uuid),
    ('dddd0000-0000-0000-0000-000000000001'::uuid, '44444444-4444-4444-4444-444444444444'::uuid)
) AS m(user_id, role_id)
WHERE NOT EXISTS (
    SELECT 1 FROM role_user ru
    WHERE ru.user_id = m.user_id
      AND ru.role_id = m.role_id
      AND ru.deleted_at IS NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM role_user WHERE user_id IN (
    'cccc0000-0000-0000-0000-000000000001',
    'dddd0000-0000-0000-0000-000000000001'
);
DELETE FROM users WHERE id IN (
    'cccc0000-0000-0000-0000-000000000001',
    'dddd0000-0000-0000-0000-000000000001'
);
-- +goose StatementEnd
