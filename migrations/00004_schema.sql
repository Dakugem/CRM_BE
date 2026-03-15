-- +goose Up

-- Products reference table
CREATE TABLE IF NOT EXISTS products (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- Roles reference table
CREATE TABLE IF NOT EXISTS roles (
    id    SERIAL PRIMARY KEY,
    name  TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL
);

-- Employees (with auth credentials)
CREATE TABLE IF NOT EXISTS employees (
    id            SERIAL PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name     TEXT NOT NULL DEFAULT '',
    email         TEXT,
    phone         TEXT,
    role          TEXT NOT NULL DEFAULT 'employee',
    photo_url     TEXT,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Clients
CREATE TABLE IF NOT EXISTS clients (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    inn        TEXT,
    email      TEXT,
    phone      TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Client Representatives
CREATE TABLE IF NOT EXISTS client_representatives (
    id         SERIAL PRIMARY KEY,
    client_id  INT  NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    full_name  TEXT NOT NULL,
    email      TEXT,
    phone      TEXT,
    position   TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Sites (площадки)
CREATE TABLE IF NOT EXISTS sites (
    id         SERIAL PRIMARY KEY,
    client_id  INT  NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    product_id INT  REFERENCES products(id) ON DELETE SET NULL,
    name       TEXT NOT NULL,
    address    TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Equipment (оборудование)
CREATE TABLE IF NOT EXISTS equipment (
    id          SERIAL PRIMARY KEY,
    product_id  INT  REFERENCES products(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    model       TEXT,
    description TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Site <-> Equipment (many-to-many with quantity)
CREATE TABLE IF NOT EXISTS site_equipment (
    site_id      INT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    equipment_id INT NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    quantity     INT NOT NULL DEFAULT 1,
    PRIMARY KEY (site_id, equipment_id)
);

-- Tickets (обращения)
CREATE TABLE IF NOT EXISTS tickets (
    id             SERIAL PRIMARY KEY,
    ticket_type    TEXT NOT NULL DEFAULT 'request',
    status         TEXT NOT NULL DEFAULT 'open',
    priority       TEXT NOT NULL DEFAULT 'medium',
    product_id     INT  REFERENCES products(id) ON DELETE SET NULL,
    description    TEXT NOT NULL DEFAULT '',
    client_id      INT  REFERENCES clients(id) ON DELETE SET NULL,
    site_id        INT  REFERENCES sites(id) ON DELETE SET NULL,
    responsible_id INT  REFERENCES employees(id) ON DELETE SET NULL,
    deadline       TIMESTAMP,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Ticket Comments
CREATE TABLE IF NOT EXISTS ticket_comments (
    id         SERIAL PRIMARY KEY,
    ticket_id  INT  NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id  INT  REFERENCES employees(id) ON DELETE SET NULL,
    text       TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Ticket History (audit log)
CREATE TABLE IF NOT EXISTS ticket_history (
    id         SERIAL PRIMARY KEY,
    ticket_id  INT  NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id  INT  REFERENCES employees(id) ON DELETE SET NULL,
    field      TEXT NOT NULL,
    old_value  TEXT,
    new_value  TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Ticket Subtasks (self-referential)
CREATE TABLE IF NOT EXISTS ticket_subtasks (
    parent_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    child_id  INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    PRIMARY KEY (parent_id, child_id),
    CONSTRAINT no_self_subtask CHECK (parent_id <> child_id)
);

-- Dashboards (per employee, with filters)
CREATE TABLE IF NOT EXISTS dashboards (
    id          SERIAL PRIMARY KEY,
    employee_id INT  NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    filters     JSONB NOT NULL DEFAULT '{}',
    position    INT  NOT NULL DEFAULT 0,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS dashboards;
DROP TABLE IF EXISTS ticket_subtasks;
DROP TABLE IF EXISTS ticket_history;
DROP TABLE IF EXISTS ticket_comments;
DROP TABLE IF EXISTS tickets;
DROP TABLE IF EXISTS site_equipment;
DROP TABLE IF EXISTS equipment;
DROP TABLE IF EXISTS sites;
DROP TABLE IF EXISTS client_representatives;
DROP TABLE IF EXISTS clients;
DROP TABLE IF EXISTS employees;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS products;
