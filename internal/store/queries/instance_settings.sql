-- name: GetInstanceSettings :one
SELECT * FROM instance_settings WHERE id = 1;

-- name: UpdateInstanceSettings :one
UPDATE instance_settings
SET weekly_digest_weekday = ?,
    weekly_digest_hour    = ?,
    monthly_digest_day    = ?,
    monthly_digest_hour   = ?
WHERE id = 1
RETURNING *;
