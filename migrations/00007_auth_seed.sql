-- +goose Up
-- +goose StatementBegin

-- Добавляем роли
INSERT INTO "auth"."Roles" ("id", "name", "description") VALUES
  (1, 'admin', 'Администратор системы'),
  (2, 'operator_ktp', 'Оператор КТП'),
  (3, 'operator_wfm', 'Оператор WFM'),
  (4, 'client_representative', 'Представитель клиента'),
  (5, 'default', 'Роль по умолчанию (без прав)')
ON CONFLICT ("id") DO NOTHING;

-- Добавляем права
INSERT INTO "auth"."Permissions" ("id", "name", "description") VALUES
  (1, 'manage_users', 'Управление пользователями'),
  (2, 'manage_clients', 'Управление клиентами'),
  (3, 'manage_tickets', 'Управление обращениями'),
  (4, 'view_reports', 'Просмотр отчетов'),
  (5, 'manage_equipment', 'Управление оборудованием'),
  (6, 'manage_sites', 'Управление площадками')
ON CONFLICT ("id") DO NOTHING;

-- Назначаем права ролям
INSERT INTO "auth"."RolePermissions" ("role_id", "permission_id") VALUES
  -- Админ имеет все права
  (1, 1), (1, 2), (1, 3), (1, 4), (1, 5), (1, 6),
  -- Оператор КТП
  (2, 2), (2, 3), (2, 5), (2, 6),
  -- Оператор WFM
  (3, 3), (3, 4),
  -- Представитель клиента
  (4, 3), (4, 4)
ON CONFLICT ("role_id", "permission_id") DO NOTHING;

-- Создаем тестового администратора
-- Пароль: admin (bcrypt hash)
-- Можно сгенерировать через: echo -n "admin" | htpasswd -bnBC 12 "" | tr -d ':\n'
INSERT INTO "auth"."Accounts" ("id", "login", "password_hash", "role_id") VALUES
  (1, 'admin', '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewY5GyYIR.ub4gO2', 1)
ON CONFLICT ("login") DO NOTHING;

-- Создаем профиль для администратора
INSERT INTO "profiles"."Profiles" ("account_id", "full_name", "phone_number", "email", "position") VALUES
  (1, 'Администратор Системы', '+7 (900) 000-00-00', 'admin@example.com', 'Системный администратор')
ON CONFLICT ("account_id") DO NOTHING;

-- Добавляем администратора в таблицу сотрудников
INSERT INTO "hrm"."Employees" ("account_id", "hire_date") VALUES
  (1, '2024-01-01')
ON CONFLICT ("account_id") DO NOTHING;

-- Создаем тестового оператора КТП
-- Пароль: operator
INSERT INTO "auth"."Accounts" ("id", "login", "password_hash", "role_id") VALUES
  (2, 'operator', '$2a$12$9XKNvF.d3TQJe4cEb.VDaO7mF0YN3lqF8rN7HfZ8mJKYZ8qN7HfZ8', 2)
ON CONFLICT ("login") DO NOTHING;

INSERT INTO "profiles"."Profiles" ("account_id", "full_name", "phone_number", "email", "position") VALUES
  (2, 'Оператор КТП', '+7 (900) 000-00-01', 'operator@example.com', 'Оператор')
ON CONFLICT ("account_id") DO NOTHING;

INSERT INTO "hrm"."Employees" ("account_id", "hire_date") VALUES
  (2, '2024-02-01')
ON CONFLICT ("account_id") DO NOTHING;

-- Создаем тестового представителя клиента
-- Пароль: client
INSERT INTO "auth"."Accounts" ("id", "login", "password_hash", "role_id") VALUES
  (3, 'client', '$2a$12$XKN7vF.d3TQJe4cEb.VDaO7mF0YN3lqF8rN7HfZ8mJKYZ8qN7HfZ9', 4)
ON CONFLICT ("login") DO NOTHING;

INSERT INTO "profiles"."Profiles" ("account_id", "full_name", "phone_number", "email", "position") VALUES
  (3, 'Представитель Клиента', '+7 (900) 000-00-02', 'client@example.com', 'Менеджер')
ON CONFLICT ("account_id") DO NOTHING;

INSERT INTO "crm"."Representatives" ("account_id") VALUES
  (3)
ON CONFLICT ("account_id") DO NOTHING;

-- Сбрасываем последовательности для ID
SELECT setval(pg_get_serial_sequence('"auth"."Accounts"', 'id'), COALESCE((SELECT MAX("id") FROM "auth"."Accounts"), 1), true);

-- +goose StatementEnd

-- +goose Down

DELETE FROM "crm"."Representatives" WHERE "account_id" IN (1, 2, 3);
DELETE FROM "hrm"."Employees" WHERE "account_id" IN (1, 2, 3);
DELETE FROM "profiles"."Profiles" WHERE "account_id" IN (1, 2, 3);
DELETE FROM "auth"."Accounts" WHERE "id" IN (1, 2, 3);
DELETE FROM "auth"."RolePermissions";
DELETE FROM "auth"."Permissions";
DELETE FROM "auth"."Roles";
