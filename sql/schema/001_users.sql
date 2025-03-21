
-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    name TEXT NOT NULL UNIQUE
);
-- +goose Down
DROP TABLE users;

-- psql "postgres://williambadgett:@localhost:5432/gator" -c "SELECT COUNT(*) FROM users;"

-- goose -dir sql/schema postgres "postgres://williambadgett:@localhost:5432/gator" down
-- goose -dir sql/schema postgres "postgres://williambadgett:@localhost:5432/gator" up

-- 2025/03/18 14:50:01 goose run: ERROR 002_users.sql: failed to run SQL migration: failed
--  to execute SQL query "CREATE TABLE users (\n    id UUID PRIMARY KEY NOT NULL,\n    created_at TIMESTAMP NOT NULL,\n
--      updated_at TIMESTAMP NOT NULL,\n    name TEXT NOT NULL UNIQUE\n);": ERROR: relation "users" already exists (SQLSTATE 42P07)