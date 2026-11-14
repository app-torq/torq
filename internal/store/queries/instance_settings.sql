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
