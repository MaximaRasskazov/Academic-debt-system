-- Начальные данные RBAC под систему академических задолженностей.
--
-- Создаются: служебный user-плейсхолдер (для FK created_by), 4 системные
-- роли (student/teacher/dean/admin) и 27 системных разрешений с
-- привязкой к ролям. Все INSERT идемпотентны (ON CONFLICT DO NOTHING),
-- чтобы повторный запуск миграции на уже инициализированной БД не падал.
--
-- Служебный UUID 00000000-... используется как created_by везде, где
-- нужна ссылка на пользователя, но реального автора нет (системный
-- посев). Сам этот аккаунт залогиниться не может — password_hash пустой.

-- +goose Up
-- +goose StatementBegin

-- 1. Служебный user-плейсхолдер.
INSERT INTO users (id, email, password_hash, first_name, last_name, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000000',
    'system@localhost.invalid',
    '',
    '__system__',
    '__system__',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;

-- 2. Системные роли.
-- level — приоритет в иерархии privilege escalation: admin > dean > teacher > student.
INSERT INTO roles (id, name, slug, description, level, is_system, created_at, created_by)
VALUES
    ('11111111-1111-1111-1111-111111111111', 'Администратор',  'admin',
     'Полный доступ ко всем операциям системы',                                                   1000, TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('22222222-2222-2222-2222-222222222222', 'Деканат',        'dean',
     'Управление пересдачами, ведомостями, отчётами, ролями ниже своего уровня',                   700, TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('33333333-3333-3333-3333-333333333333', 'Преподаватель',  'teacher',
     'Выставление долгов и оценок по своим дисциплинам, участие в пересдачах',                     300, TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('44444444-4444-4444-4444-444444444444', 'Студент',        'student',
     'Просмотр собственных долгов и пересдач, подача заявки на роль преподавателя',                100, TRUE, NOW(), '00000000-0000-0000-0000-000000000000')
ON CONFLICT (id) DO NOTHING;

-- 3. Системные разрешения.
-- UUID детерминированы по группам:
--   a0000001-* — debts.*
--   a0000002-* — retakes.*
--   a0000003-* — teacher_request.*
--   a0000004-* — users.*
--   a0000005-* — roles.*
--   a0000006-* — disciplines.*
--   a0000007-* — audit.* / changelog.*
--   a0000008-* — reports.*
INSERT INTO permissions (id, name, slug, description, is_system, created_at, created_by)
VALUES
    -- Академические долги
    ('a0000001-0000-0000-0000-000000000001', 'Debts: View Own',           'debts.view.own',           'Студент: просмотр собственных долгов',                                 TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000001-0000-0000-0000-000000000002', 'Debts: View By Discipline', 'debts.view.by_discipline', 'Преподаватель: просмотр долгов по своим дисциплинам',                  TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000001-0000-0000-0000-000000000003', 'Debts: View All',           'debts.view.all',           'Деканат/админ: просмотр всех долгов',                                  TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000001-0000-0000-0000-000000000004', 'Debts: Create',             'debts.create',             'Выставление долга студенту',                                            TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000001-0000-0000-0000-000000000005', 'Debts: Update',             'debts.update',             'Изменение долга, в том числе перевод в оценку',                          TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000001-0000-0000-0000-000000000006', 'Debts: Delete',             'debts.delete',             'Аннулирование долга',                                                   TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),

    -- Пересдачи
    ('a0000002-0000-0000-0000-000000000001', 'Retakes: View Own',         'retakes.view.own',         'Просмотр пересдач, в которых пользователь является участником',         TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000002-0000-0000-0000-000000000002', 'Retakes: View All',         'retakes.view.all',         'Просмотр всех пересдач',                                                TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000002-0000-0000-0000-000000000003', 'Retakes: Create',           'retakes.create',           'Создание пересдачи (деканат)',                                          TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000002-0000-0000-0000-000000000004', 'Retakes: Request',          'retakes.request',          'Подача преподавателем заявки на создание пересдачи',                   TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000002-0000-0000-0000-000000000005', 'Retakes: Update',           'retakes.update',           'Изменение пересдачи (деканат)',                                         TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000002-0000-0000-0000-000000000006', 'Retakes: Request Change',   'retakes.request_change',   'Подача заявки на изменение времени/места пересдачи',                    TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000002-0000-0000-0000-000000000007', 'Retakes: Approve Change',   'retakes.approve_change',   'Одобрение/отклонение заявки на изменение пересдачи',                    TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000002-0000-0000-0000-000000000008', 'Retakes: Assign Grade',     'retakes.assign_grade',     'Выставление оценки по результату пересдачи (закрытие долга)',           TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),

    -- Заявки на роль преподавателя
    ('a0000003-0000-0000-0000-000000000001', 'Teacher Request: Create',   'teacher_request.create',   'Подача заявки на получение роли преподавателя',                         TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000003-0000-0000-0000-000000000002', 'Teacher Request: Review',   'teacher_request.review',   'Рассмотрение заявок на роль преподавателя',                              TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),

    -- Управление пользователями
    ('a0000004-0000-0000-0000-000000000001', 'Users: View',               'users.view',               'Просмотр списка пользователей',                                          TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000004-0000-0000-0000-000000000002', 'Users: Update',             'users.update',             'Изменение данных пользователя администратором',                          TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000004-0000-0000-0000-000000000003', 'Users: Delete',             'users.delete',             'Деактивация пользователя',                                              TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),

    -- Назначение ролей
    ('a0000005-0000-0000-0000-000000000001', 'Roles: Assign',             'roles.assign',             'Выдача/отзыв ролей пользователям',                                       TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),

    -- Справочник дисциплин
    ('a0000006-0000-0000-0000-000000000001', 'Disciplines: View',         'disciplines.view',         'Просмотр справочника дисциплин',                                         TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000006-0000-0000-0000-000000000002', 'Disciplines: Create',       'disciplines.create',       'Создание дисциплины',                                                    TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000006-0000-0000-0000-000000000003', 'Disciplines: Update',       'disciplines.update',       'Обновление дисциплины',                                                  TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000006-0000-0000-0000-000000000004', 'Disciplines: Delete',       'disciplines.delete',       'Удаление дисциплины',                                                    TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),

    -- Системные журналы и история
    ('a0000007-0000-0000-0000-000000000001', 'Audit: View',               'audit.view',               'Просмотр журнала аудита',                                                TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),
    ('a0000007-0000-0000-0000-000000000002', 'Changelog: View',           'changelog.view',           'Просмотр истории изменений сущностей',                                   TRUE, NOW(), '00000000-0000-0000-0000-000000000000'),

    -- Отчёты
    ('a0000008-0000-0000-0000-000000000001', 'Reports: Export',           'reports.export',           'Экспорт сводных отчётов в XLSX/CSV',                                     TRUE, NOW(), '00000000-0000-0000-0000-000000000000')
ON CONFLICT (id) DO NOTHING;

-- 4. Привязка разрешений к ролям.
-- Админ получает ВСЕ системные разрешения автоматически — это гарантирует,
-- что при добавлении новой permission в будущей миграции она автоматически
-- появится у админа (без отдельного INSERT).
INSERT INTO permission_role (role_id, permission_id, created_at, created_by)
SELECT
    '11111111-1111-1111-1111-111111111111',
    p.id,
    NOW(),
    '00000000-0000-0000-0000-000000000000'
FROM permissions p
WHERE p.is_system = TRUE
  AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM permission_role pr
    WHERE pr.role_id = '11111111-1111-1111-1111-111111111111'
      AND pr.permission_id = p.id
      AND pr.deleted_at IS NULL
  );

-- Деканат: управление пересдачами, ведомостями, ролями, отчётами.
INSERT INTO permission_role (role_id, permission_id, created_at, created_by)
SELECT
    '22222222-2222-2222-2222-222222222222',
    p.id,
    NOW(),
    '00000000-0000-0000-0000-000000000000'
FROM permissions p
WHERE p.slug IN (
        'debts.view.all',
        'debts.delete',
        'retakes.view.all',
        'retakes.create',
        'retakes.update',
        'retakes.approve_change',
        'teacher_request.review',
        'users.view',
        'roles.assign',
        'disciplines.view',
        'disciplines.create',
        'disciplines.update',
        'reports.export'
    )
  AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM permission_role pr
    WHERE pr.role_id = '22222222-2222-2222-2222-222222222222'
      AND pr.permission_id = p.id
      AND pr.deleted_at IS NULL
  );

-- Преподаватель: выставление долгов и оценок, участие в пересдачах.
INSERT INTO permission_role (role_id, permission_id, created_at, created_by)
SELECT
    '33333333-3333-3333-3333-333333333333',
    p.id,
    NOW(),
    '00000000-0000-0000-0000-000000000000'
FROM permissions p
WHERE p.slug IN (
        'debts.view.by_discipline',
        'debts.create',
        'debts.update',
        'retakes.view.own',
        'retakes.request',
        'retakes.request_change',
        'retakes.assign_grade',
        'disciplines.view'
    )
  AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM permission_role pr
    WHERE pr.role_id = '33333333-3333-3333-3333-333333333333'
      AND pr.permission_id = p.id
      AND pr.deleted_at IS NULL
  );

-- Студент: просмотр своих долгов и пересдач, заявка на роль преподавателя.
INSERT INTO permission_role (role_id, permission_id, created_at, created_by)
SELECT
    '44444444-4444-4444-4444-444444444444',
    p.id,
    NOW(),
    '00000000-0000-0000-0000-000000000000'
FROM permissions p
WHERE p.slug IN (
        'debts.view.own',
        'retakes.view.own',
        'teacher_request.create'
    )
  AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM permission_role pr
    WHERE pr.role_id = '44444444-4444-4444-4444-444444444444'
      AND pr.permission_id = p.id
      AND pr.deleted_at IS NULL
  );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM permission_role WHERE role_id IN (
    '11111111-1111-1111-1111-111111111111',
    '22222222-2222-2222-2222-222222222222',
    '33333333-3333-3333-3333-333333333333',
    '44444444-4444-4444-4444-444444444444'
);
DELETE FROM permissions WHERE is_system = TRUE;
DELETE FROM roles WHERE is_system = TRUE;
DELETE FROM users WHERE id = '00000000-0000-0000-0000-000000000000';
-- +goose StatementEnd
