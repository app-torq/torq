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

-- name: UpdateInstanceDefaults :one
UPDATE instance_settings
SET default_date_format       = ?,
    default_number_format     = ?,
    default_time_format       = ?,
    default_currency_code     = ?,
    default_power_unit        = ?,
    default_distance_unit     = ?,
    default_volume_unit       = ?,
    default_torque_unit       = ?,
    default_pressure_unit     = ?,
    default_displacement_unit = ?
WHERE id = 1
RETURNING *;

-- name: UpdateInstanceBranding :one
UPDATE instance_settings
SET brand_name    = ?,
    brand_address = ?,
    brand_phone   = ?,
    brand_email   = ?,
    brand_website = ?
WHERE id = 1
RETURNING *;

-- name: GetInstanceLogo :one
SELECT * FROM instance_logo WHERE id = 1;

-- name: GetInstanceLogoUpdatedAt :one
SELECT updated_at FROM instance_logo WHERE id = 1;

-- name: PutInstanceLogo :exec
INSERT INTO instance_logo (id, mime_type, data, updated_at) VALUES (1, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET mime_type = excluded.mime_type, data = excluded.data, updated_at = excluded.updated_at;

-- name: DeleteInstanceLogo :exec
DELETE FROM instance_logo WHERE id = 1;

-- name: SetPublicLinksEnabled :one
UPDATE instance_settings SET public_links_enabled = ? WHERE id = 1 RETURNING *;
