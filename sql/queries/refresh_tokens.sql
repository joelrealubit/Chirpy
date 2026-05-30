-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
        token,
        created_at,
        updated_at,
        expires_at,
        revoked_at,
        user_id
    )
VALUES(
        $1,
        NOW(),
        NOW(),
        $2,
        $3,
        $4
    )
RETURNING *;
-- name: GetLatestRefreshToken :one
SELECT *
FROM refresh_tokens
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;