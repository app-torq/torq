-- name: CreateHourMeter :one
INSERT INTO hour_meters (vehicle_id, name, current_hours_tenths, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetHourMeter :one
SELECT * FROM hour_meters WHERE id = ?;

-- name: ListHourMetersForVehicles :many
SELECT * FROM hour_meters WHERE vehicle_id IN (sqlc.slice(ids)) ORDER BY vehicle_id, id;

-- name: ListAllHourMeters :many
SELECT * FROM hour_meters ORDER BY vehicle_id, id;

-- name: RenameHourMeter :one
UPDATE hour_meters SET name = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: SetHourMeterCurrent :exec
UPDATE hour_meters SET current_hours_tenths = ?, updated_at = ? WHERE id = ?;

-- name: DeleteHourMeter :exec
DELETE FROM hour_meters WHERE id = ?;

-- name: CountHourMeterEntryReadings :one
SELECT CAST(COUNT(service_log_id) AS INTEGER) AS service_logs, CAST(COUNT(repair_log_id) AS INTEGER) AS repairs
FROM hour_meter_readings WHERE hour_meter_id = ?;

-- name: InsertHourMeterReading :one
INSERT INTO hour_meter_readings (hour_meter_id, reading_date, hours_tenths, source, service_log_id, repair_log_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: ListHourMeterReadings :many
SELECT id, hour_meter_id, reading_date, hours_tenths, source, created_at FROM hour_meter_readings
WHERE hour_meter_id = sqlc.arg(hour_meter_id)
  AND (sqlc.narg(from_date) IS NULL OR reading_date >= sqlc.narg(from_date))
  AND (sqlc.narg(to_date) IS NULL OR reading_date <= sqlc.narg(to_date))
ORDER BY reading_date DESC, hours_tenths DESC, id DESC
LIMIT 500;

-- name: ListServiceLogMeterReadings :many
SELECT service_log_id, hour_meter_id, hours_tenths FROM hour_meter_readings
WHERE service_log_id IN (sqlc.slice(ids))
ORDER BY service_log_id, hour_meter_id;

-- name: ListRepairMeterReadings :many
SELECT repair_log_id, hour_meter_id, hours_tenths FROM hour_meter_readings
WHERE repair_log_id IN (sqlc.slice(ids))
ORDER BY repair_log_id, hour_meter_id;

-- name: DeleteServiceLogMeterReadings :exec
DELETE FROM hour_meter_readings WHERE service_log_id = ?;

-- name: DeleteRepairMeterReadings :exec
DELETE FROM hour_meter_readings WHERE repair_log_id = ?;

-- name: CountRemindersOnlyByHours :one
SELECT CAST(COUNT(*) AS INTEGER) FROM reminders
WHERE hour_meter_id = ? AND interval_km IS NULL AND interval_months IS NULL AND interval_days IS NULL;

-- name: ClearRemindersHourMeter :exec
-- Reminders that also have a distance or time interval drop their hours dimension.
UPDATE reminders SET hour_meter_id = NULL, interval_hours = NULL, lead_hours = NULL, start_hours_tenths = NULL,
    last_done_hours_tenths = NULL, next_due_hours_tenths = NULL, updated_at = ?
WHERE hour_meter_id = ?;
