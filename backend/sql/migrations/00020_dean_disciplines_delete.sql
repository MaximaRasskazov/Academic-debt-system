-- Добавляем disciplines.delete деканату.
--
-- По результатам тестирования выяснилось, что роутер ограничивал
-- soft-delete и restore дисциплин только ролью admin, хотя по
-- бизнес-смыслу деканат должен иметь эту возможность наравне с admin.
-- Тест-план (BACK-01 QA) тоже проверяет DELETE/restore от имени dean.

-- +goose Up
-- +goose StatementBegin
INSERT INTO permission_role (role_id, permission_id, created_at, created_by)
SELECT
    '22222222-2222-2222-2222-222222222222',
    p.id,
    NOW(),
    '00000000-0000-0000-0000-000000000000'
FROM permissions p
WHERE p.slug = 'disciplines.delete'
  AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM permission_role pr
    WHERE pr.role_id = '22222222-2222-2222-2222-222222222222'
      AND pr.permission_id = p.id
      AND pr.deleted_at IS NULL
  );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM permission_role
WHERE role_id = '22222222-2222-2222-2222-222222222222'
  AND permission_id = (
    SELECT id FROM permissions WHERE slug = 'disciplines.delete' LIMIT 1
  );
-- +goose StatementEnd
