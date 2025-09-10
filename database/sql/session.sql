-- name: AddSession :one
INSERT INTO sessions (token, username, expires_at)
VALUES ($1, $2, $3)
RETURNING id, token, username, expires_at, created_at, updated_at;

-- name: GetSessionByToken :one
SELECT id, token, username, expires_at, created_at, updated_at
FROM sessions WHERE token = $1;

-- name: DeleteSessionByToken :exec
DELETE FROM sessions WHERE token = $1;