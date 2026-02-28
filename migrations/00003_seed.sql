-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN

INSERT INTO users (username, password) VALUES
    ('admin',   '21232f297a57a5a743894a0e4a801fc3'),  -- password: admin
    ('manager', '5f4dcc3b5aa765d61d8327deb882cf99')   -- password: password
ON CONFLICT (username) DO NOTHING;

-- Appeals
INSERT INTO appeals (type, status, criticality, product, description, client, venue, responsible, deadline, updated) VALUES
    ('complaint', 'open',     'high',   'Продукт А', 'Не работает авторизация',          'ООО Ромашка',  'Москва',          'admin',   NOW() + INTERVAL '3 days',  NOW()),
    ('request',   'pending',  'medium', 'Продукт Б', 'Добавить экспорт в Excel',          'ИП Иванов',    'Санкт-Петербург', 'manager', NOW() + INTERVAL '7 days',  NOW()),
    ('incident',  'open',     'high',   'Продукт А', 'Сервер недоступен с 09:00',         'АО Техносфера','Казань',          'admin',   NOW() + INTERVAL '1 days',  NOW()),
    ('request',   'closed',   'low',    'Продукт В', 'Обновить документацию',             'ООО Прогресс', 'Новосибирск',     'manager', NOW() - INTERVAL '2 days',  NOW()),
    ('complaint', 'pending',  'medium', 'Продукт Б', 'Неверно считается итоговая сумма',  'ИП Петрова',   'Екатеринбург',    'manager', NOW() + INTERVAL '5 days',  NOW());

END $$;
-- +goose StatementEnd

-- +goose Down
DELETE FROM appeals;
DELETE FROM users WHERE username IN ('admin', 'manager');
