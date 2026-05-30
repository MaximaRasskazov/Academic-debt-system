-- Минимальные демо-аккаунты для разработки и тестов фронта/QA.
--
-- Накатываются ОТДЕЛЬНОЙ цепочкой миграций (goose-таблица
-- goose_seed_version, см. entrypoint.sh) — это позволяет легко
-- отключать сиды в prod через SEED_DEV_ACCOUNTS=false, не вмешиваясь
-- в основную последовательность миграций sql/migrations.
--
-- Здесь сидятся ТОЛЬКО admin и dean — этих ролей в эмуляторе деканата
-- нет (там только student/teacher), а нам они нужны чтобы залогиниться
-- в систему и попасть в админ-панель. Студенты и преподаватели
-- приходят целиком из эмулятора через sync.Service.
--
-- Если убрать и эти два сида — в свежей БД ВООБЩЕ не будет ни одного
-- администратора, и попасть в /api/users, /api/sync/trigger и т.д.
-- будет некому. Создание admin без сидов потребует ручной SQL-вставки
-- после деплоя — это худший UX.
--
-- Состав:
--   admin@academic.local  — роль admin (level 1000)
--   dean@academic.local   — роль dean  (level 700)
--
-- Пароль у обоих одинаковый: "password" (bcrypt cost=10).
-- ID детерминированные, чтобы повторный накат на инициализированной БД
-- через ON CONFLICT (id) DO NOTHING не дублировал данные.

-- +goose Up
-- +goose StatementBegin

INSERT INTO users (id, email, password_hash, first_name, last_name, middle_name, group_name)
VALUES
    ('aaaa0000-0000-0000-0000-000000000001', 'admin@academic.local', '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Главный', 'Администратор', NULL,        NULL),
    ('bbbb0000-0000-0000-0000-000000000001', 'dean@academic.local',  '$2a$10$gMK4lndQ1uTZ0CpVXyqNC.ZB5SAU8QEchkAD81B73OENQv7FQOF1.', 'Иван',    'Деканов',       'Сергеевич', NULL)
ON CONFLICT (id) DO NOTHING;

-- Привязка ролей. created_by — служебный системный user из 00010_seed_rbac.
INSERT INTO role_user (user_id, role_id, created_by)
SELECT m.user_id, m.role_id, '00000000-0000-0000-0000-000000000000'::uuid
FROM (VALUES
    ('aaaa0000-0000-0000-0000-000000000001'::uuid, '11111111-1111-1111-1111-111111111111'::uuid),
    ('bbbb0000-0000-0000-0000-000000000001'::uuid, '22222222-2222-2222-2222-222222222222'::uuid)
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
    'bbbb0000-0000-0000-0000-000000000001'
);
DELETE FROM users WHERE id IN (
    'aaaa0000-0000-0000-0000-000000000001',
    'bbbb0000-0000-0000-0000-000000000001'
);
-- +goose StatementEnd
