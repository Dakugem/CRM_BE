-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN

-- Products
INSERT INTO products (name) VALUES
    ('МКД'),
    ('Интернет'),
    ('IP-телефония')
ON CONFLICT (name) DO NOTHING;

-- Roles
INSERT INTO roles (name, label) VALUES
    ('admin',    'Администратор'),
    ('manager',  'Менеджер'),
    ('engineer', 'Инженер'),
    ('employee', 'Сотрудник')
ON CONFLICT (name) DO NOTHING;

-- Migrate existing users to employees
INSERT INTO employees (username, password_hash, full_name, role)
SELECT username, password, username, 'admin'
FROM users WHERE username = 'admin'
ON CONFLICT (username) DO NOTHING;

INSERT INTO employees (username, password_hash, full_name, role)
SELECT username, password, username, 'manager'
FROM users WHERE username = 'manager'
ON CONFLICT (username) DO NOTHING;

END $$;
-- +goose StatementEnd

-- +goose Down
DELETE FROM employees WHERE username IN ('admin', 'manager');
DELETE FROM roles;
DELETE FROM products;
