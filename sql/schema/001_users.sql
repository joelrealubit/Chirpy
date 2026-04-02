-- +goose Up
CREATE TABLE users (
    id INT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    email TEXT NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE users;