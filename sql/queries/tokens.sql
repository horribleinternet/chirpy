-- name: AddRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at)
SELECT $1 AS token, NOW() AS created_at, NOW() AS updated_at, id AS user_id, NOW() + INTERVAL '60 DAYS' AS expires_at
FROM users WHERE email = $2
RETURNING token;

-- name: GetUserByToken :one
SELECT user_id, expires_at FROM refresh_tokens WHERE token = $1;

-- name: RevokeToken :exec
UPDATE refresh_tokens SET updated_at = NOW(), revoked_at = NOW() WHERE token = $1;
