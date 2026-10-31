-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateUser :one
INSERT INTO users (username, display_name, role, locale, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: ListUsers :many
SELECT * FROM users ORDER BY username COLLATE NOCASE;

-- name: UpdateUserProfile :one
UPDATE users SET username = ?, display_name = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: UpdateUserPreferences :one
UPDATE users
SET display_name = ?, locale = ?, date_format = ?, number_format = ?,
    currency_code = ?, currency_symbol = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: SetUserRole :one
UPDATE users SET role = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: SetUserDisabled :one
UPDATE users SET disabled_at = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;

-- name: CountEnabledAdmins :one
SELECT COUNT(*) FROM users WHERE role = 'admin' AND disabled_at IS NULL;

-- name: CreateLocalIdentity :exec
INSERT INTO user_identities (user_id, provider, subject, password_hash, created_at)
VALUES (?, 'local', ?, ?, ?);

-- name: GetLocalIdentityByUsername :one
SELECT i.id AS identity_id, i.password_hash, u.id AS user_id
FROM user_identities i
JOIN users u ON u.id = i.user_id
WHERE i.provider = 'local' AND i.subject = ? AND u.disabled_at IS NULL;

-- name: GetLocalIdentityByUser :one
SELECT * FROM user_identities WHERE user_id = ? AND provider = 'local';

-- name: UpdatePasswordHash :exec
UPDATE user_identities SET password_hash = ? WHERE user_id = ? AND provider = 'local';

-- name: UpdateLocalSubject :exec
UPDATE user_identities SET subject = ? WHERE user_id = ? AND provider = 'local';

-- name: TouchLastLogin :exec
UPDATE user_identities SET last_login_at = ? WHERE id = ?;
