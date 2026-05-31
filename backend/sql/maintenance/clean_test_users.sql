-- clean_test_users.sql — разовая очистка тестового мусора из БД.
--
-- Интеграционные тесты создают пользователей с email вида
-- '<prefix>+<timestamp>@test.local'. Начиная с этого фикса они
-- убирают за собой сами (per-test CleanupUser + TestMain-дочистка,
-- см. internal/repo/testsupport.go). Но если в БД уже накопился мусор
-- от старых прогонов — прогоните этот скрипт один раз.
--
-- Безопасность: фильтр по '@test.local' не затрагивает dev-сиды
-- (admin@academic.local, dean@academic.local) и реальные/эмуляторные
-- данные. Удаляет только синтетических тестовых юзеров и записи,
-- которые на них ссылаются (audit_log, change_logs, debts и т.д.).
--
-- Запуск:
--   docker compose exec -T postgres \
--     psql -U academic -d academic_debts -f - < backend/sql/maintenance/clean_test_users.sql
-- либо скопировать содержимое в psql-сессию.

BEGIN;

-- id всех тестовых юзеров — во временную таблицу, чтобы не повторять
-- подзапрос и не словить рассинхрон между DELETE'ами.
CREATE TEMP TABLE _test_users ON COMMIT DROP AS
    SELECT id FROM users WHERE email LIKE '%@test.local';

-- Порядок важен: сначала таблицы с FK на users (RESTRICT/NO ACTION),
-- затем сами users. CASCADE-таблицы (access_tokens, notifications,
-- role_user.user_id) уберутся автоматически при удалении users.
DELETE FROM retake_participants
    WHERE user_id IN (SELECT id FROM _test_users)
       OR graded_by IN (SELECT id FROM _test_users);

DELETE FROM retake_change_requests
    WHERE requested_by IN (SELECT id FROM _test_users)
       OR reviewed_by IN (SELECT id FROM _test_users);

DELETE FROM retakes
    WHERE created_by IN (SELECT id FROM _test_users)
       OR deleted_by IN (SELECT id FROM _test_users);

DELETE FROM debts
    WHERE student_id IN (SELECT id FROM _test_users)
       OR issued_by IN (SELECT id FROM _test_users)
       OR graded_by IN (SELECT id FROM _test_users)
       OR deleted_by IN (SELECT id FROM _test_users);

DELETE FROM change_logs
    WHERE created_by IN (SELECT id FROM _test_users);

DELETE FROM audit_log
    WHERE actor_id IN (SELECT id FROM _test_users);

DELETE FROM teacher_role_requests
    WHERE requested_by IN (SELECT id FROM _test_users)
       OR reviewed_by IN (SELECT id FROM _test_users);

DELETE FROM teacher_disciplines
    WHERE teacher_id IN (SELECT id FROM _test_users)
       OR created_by IN (SELECT id FROM _test_users)
       OR deleted_by IN (SELECT id FROM _test_users);

DELETE FROM disciplines
    WHERE created_by IN (SELECT id FROM _test_users)
       OR deleted_by IN (SELECT id FROM _test_users);

DELETE FROM users
    WHERE id IN (SELECT id FROM _test_users);

-- Должно быть 0.
SELECT count(*) AS remaining_test_users
    FROM users WHERE email LIKE '%@test.local';

COMMIT;
