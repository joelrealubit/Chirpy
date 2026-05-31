-- name: CreateUser :one
INSERT INTO users (
        id,
        created_at,
        updated_at,
        email,
        userpassword
    )
VALUES (
        gen_random_uuid (),
        NOW(),
        NOW(),
        $1,
        $2
    )
RETURNING *;
-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;
-- name: GetUserById :one
SELECT *
FROM users
WHERE id = $1;
-- name: DeleteUsers :exec
DELETE FROM users;