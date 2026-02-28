-- +goose Up
CREATE TABLE IF NOT EXISTS appeals (
    id           SERIAL PRIMARY KEY,
    type         TEXT NOT NULL,
    status       TEXT NOT NULL,
    criticality  TEXT NOT NULL,
    product      TEXT NOT NULL,
    description  TEXT NOT NULL,
    client       TEXT NOT NULL,
    venue        TEXT NOT NULL,
    responsible  TEXT NOT NULL,
    deadline     TIMESTAMP NOT NULL,
    updated      TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS appeals;
