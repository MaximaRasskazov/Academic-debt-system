-- Демо-аккаунты для разработки и тестов фронта/QA.
--
-- Накатываются ОТДЕЛЬНОЙ цепочкой миграций (goose-таблица
-- goose_seed_version, см. entrypoint.sh) — это позволяет легко
-- отключать сиды в prod через SEED_DEV_ACCOUNTS=false, не вмешиваясь
-- в основную последовательность миграций sql/migrations.
--
-- Состав:
--   admin@academic.local        — роль admin (level 1000)
--   dean@academic.local         — роль dean (700)
--   teacher1@academic.local     — роль teacher (300)
--   teacher2@academic.local     — роль teacher
--   student1@academic.local     — роль student (100), группа БСБО-01-22
--   student2@academic.local     — роль student, группа БСБО-01-22
--   student3@academic.local     — роль student, группа БСБО-02-22
--
-- У всех пароль одинаковый: "password" (bcrypt cost=10).
--
-- ID детерминированные, чтобы повторный накат на инициализированной БД
-- через ON CONFLICT (id) DO NOTHING не дублировал данные.

-- +goose Up
-- +goose StatementBegin

INSERT INTO users (id, email, password_hash, first_name, last_name, middle_name, group_name)
VALUES
    ('aaaa0000-0000-0000-0000-000000000001', 'admin@academic.local',    '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Главный',  'Администратор', NULL,         NULL),
    ('bbbb0000-0000-0000-0000-000000000001', 'dean@academic.local',     '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Иван',     'Деканов',       'Сергеевич',  NULL),
    ('cccc0000-0000-0000-0000-000000000001', 'teacher1@academic.local', '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Пётр',     'Преподов',      'Николаевич', NULL),
    ('cccc0000-0000-0000-0000-000000000002', 'teacher2@academic.local', '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Мария',    'Лекторова',     'Андреевна',  NULL),
    ('dddd0000-0000-0000-0000-000000000001', 'student1@academic.local', '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Алексей',  'Студентов',     'Иванович',   'БСБО-01-22'),
    ('dddd0000-0000-0000-0000-000000000002', 'student2@academic.local', '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Елена',    'Группова',      'Петровна',   'БСБО-01-22'),
    ('dddd0000-0000-0000-0000-000000000003', 'student3@academic.local', '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Дмитрий',  'Зачётов',       NULL,         'БСБО-02-22')
ON CONFLICT (id) DO NOTHING;

-- Привязка ролей. created_by — служебный системный user из 00010_seed_rbac.
-- WHERE NOT EXISTS защищает от двойной выдачи (UNIQUE-индекс по
-- (user_id, role_id) WHERE deleted_at IS NULL).
INSERT INTO role_user (user_id, role_id, created_by)
SELECT m.user_id, m.role_id, '00000000-0000-0000-0000-000000000000'::uuid
FROM (VALUES
    ('aaaa0000-0000-0000-0000-000000000001'::uuid, '11111111-1111-1111-1111-111111111111'::uuid),
    ('bbbb0000-0000-0000-0000-000000000001'::uuid, '22222222-2222-2222-2222-222222222222'::uuid),
    ('cccc0000-0000-0000-0000-000000000001'::uuid, '33333333-3333-3333-3333-333333333333'::uuid),
    ('cccc0000-0000-0000-0000-000000000002'::uuid, '33333333-3333-3333-3333-333333333333'::uuid),
    ('dddd0000-0000-0000-0000-000000000001'::uuid, '44444444-4444-4444-4444-444444444444'::uuid),
    ('dddd0000-0000-0000-0000-000000000002'::uuid, '44444444-4444-4444-4444-444444444444'::uuid),
    ('dddd0000-0000-0000-0000-000000000003'::uuid, '44444444-4444-4444-4444-444444444444'::uuid)
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
    'aaaa0000-0000-0000-0000-000000000001',
    'bbbb0000-0000-0000-0000-000000000001',
    'cccc0000-0000-0000-0000-000000000001',
    'cccc0000-0000-0000-0000-000000000002',
    'dddd0000-0000-0000-0000-000000000001',
    'dddd0000-0000-0000-0000-000000000002',
    'dddd0000-0000-0000-0000-000000000003'
);
DELETE FROM users WHERE id IN (
    'aaaa0000-0000-0000-0000-000000000001',
    'bbbb0000-0000-0000-0000-000000000001',
    'cccc0000-0000-0000-0000-000000000001',
    'cccc0000-0000-0000-0000-000000000002',
    'dddd0000-0000-0000-0000-000000000001',
    'dddd0000-0000-0000-0000-000000000002',
    'dddd0000-0000-0000-0000-000000000003'
);
-- +goose StatementEnd
