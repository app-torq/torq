-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, created_at, last_seen_at, expires_at, user_agent, ip, device_name)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetSessionByTokenHash :one
SELECT sqlc.embed(sessions), sqlc.embed(users)
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = ? AND users.disabled_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = ?, expires_at = ?, ip = ? WHERE id = ?;

-- name: ListUserSessions :many
SELECT * FROM sessions
WHERE user_id = ? AND expires_at > ?
ORDER BY last_seen_at DESC, id DESC;

-- name: DeleteUserSession :execrows
DELETE FROM sessions WHERE id = ? AND user_id = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = ?;

-- name: DeleteUserSessionsExcept :exec
DELETE FROM sessions WHERE user_id = ? AND id <> ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= ?;
