-- name: GetTOTP :one
SELECT * FROM user_totp WHERE user_id = ?;

-- name: UpsertPendingTOTP :exec
-- A new setup replaces an unconfirmed one; a confirmed one is never replaced here.
INSERT INTO user_totp (user_id, secret_enc, created_at) VALUES (?, ?, ?)
ON CONFLICT (user_id) DO UPDATE SET secret_enc = excluded.secret_enc, created_at = excluded.created_at, last_step = 0
WHERE user_totp.confirmed_at IS NULL;

-- name: ConfirmTOTP :exec
UPDATE user_totp SET confirmed_at = ?, last_step = ? WHERE user_id = ?;

-- name: SetTOTPStep :execrows
-- Only moves forward, so two requests with the same code can't both win.
UPDATE user_totp SET last_step = sqlc.arg(step) WHERE user_id = sqlc.arg(user_id) AND last_step < sqlc.arg(step);

-- name: DeleteTOTP :exec
DELETE FROM user_totp WHERE user_id = ?;

-- name: DeleteRecoveryCodes :exec
DELETE FROM user_recovery_codes WHERE user_id = ?;

-- name: InsertRecoveryCode :exec
INSERT INTO user_recovery_codes (user_id, code_hash, created_at) VALUES (?, ?, ?);

-- name: UseRecoveryCode :execrows
UPDATE user_recovery_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at IS NULL;

-- name: CountRecoveryCodes :one
SELECT COUNT(*) FROM user_recovery_codes WHERE user_id = ? AND used_at IS NULL;

-- name: GetWebAuthnHandle :one
SELECT handle FROM user_webauthn WHERE user_id = ?;

-- name: InsertWebAuthnHandle :exec
INSERT INTO user_webauthn (user_id, handle) VALUES (?, ?);

-- name: GetUserByWebAuthnHandle :one
SELECT user_id FROM user_webauthn WHERE handle = ?;

-- name: ListPasskeys :many
SELECT * FROM user_passkeys WHERE user_id = ? ORDER BY created_at, id;

-- name: GetPasskey :one
SELECT * FROM user_passkeys WHERE id = ? AND user_id = ?;

-- name: InsertPasskey :one
INSERT INTO user_passkeys (user_id, credential_id, credential, name, user_verified, created_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdatePasskeyUse :exec
UPDATE user_passkeys SET credential = ?, last_used_at = ? WHERE id = ?;

-- name: RenamePasskey :one
UPDATE user_passkeys SET name = ? WHERE id = ? AND user_id = ? RETURNING *;

-- name: DeletePasskey :execrows
DELETE FROM user_passkeys WHERE id = ? AND user_id = ?;

-- name: CountFactors :one
-- Confirmed second factors of a user: an authenticator counts once, each passkey once.
SELECT CAST((SELECT COUNT(*) FROM user_totp t WHERE t.user_id = sqlc.arg(user_id) AND t.confirmed_at IS NOT NULL)
          + (SELECT COUNT(*) FROM user_passkeys p WHERE p.user_id = sqlc.arg(user_id)) AS INTEGER);

-- name: ListUsersWithFactors :many
SELECT user_id FROM user_totp WHERE confirmed_at IS NOT NULL
UNION
SELECT user_id FROM user_passkeys;

-- name: DeletePasskeys :exec
DELETE FROM user_passkeys WHERE user_id = ?;

-- name: DeleteWebAuthnHandle :exec
DELETE FROM user_webauthn WHERE user_id = ?;

-- name: SetUserMFARequired :exec
UPDATE users SET mfa_required = ?, updated_at = ? WHERE id = ?;
