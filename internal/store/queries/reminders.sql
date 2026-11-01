-- name: CreateReminder :one
INSERT INTO reminders (vehicle_id, service_category_id, title, interval_km, interval_months, interval_days,
                       lead_km, lead_days, start_date, start_odometer_km, state_changed_at, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReminder :one
SELECT * FROM reminders WHERE id = ?;

-- name: ListReminders :many
SELECT r.* FROM reminders r
JOIN vehicles v ON v.id = r.vehicle_id
WHERE (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR r.vehicle_id = sqlc.arg(vehicle_id))
  AND (CAST(sqlc.arg(state) AS TEXT) = '' OR r.state = sqlc.arg(state))
  AND (CAST(sqlc.arg(include_paused) AS INTEGER) = 1 OR r.paused_at IS NULL)
  AND (CAST(sqlc.arg(include_archived) AS INTEGER) = 1 OR v.archived_at IS NULL)
ORDER BY r.id;

-- name: ListRemindersForVehicle :many
SELECT * FROM reminders WHERE vehicle_id = ? ORDER BY id;

-- name: UpdateReminder :one
UPDATE reminders
SET service_category_id = ?, title = ?, interval_km = ?, interval_months = ?, interval_days = ?,
    lead_km = ?, lead_days = ?, start_date = ?, start_odometer_km = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteReminder :exec
DELETE FROM reminders WHERE id = ?;

-- name: SetReminderPaused :one
UPDATE reminders SET paused_at = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: SetReminderDone :exec
UPDATE reminders SET last_done_date = ?, last_done_odometer_km = ?, updated_at = ? WHERE id = ?;

-- name: SetReminderEvaluation :exec
UPDATE reminders
SET last_done_date = ?, last_done_odometer_km = ?, next_due_km = ?, next_due_date = ?,
    state = ?, state_trigger = ?, state_changed_at = ?, last_notified_state = ?
WHERE id = ?;

-- name: ListVehicleIDsWithReminders :many
SELECT DISTINCT vehicle_id FROM reminders ORDER BY vehicle_id;

-- name: LatestServiceForCategory :one
SELECT s.service_date, s.odometer_km
FROM service_log_items i
JOIN service_logs s ON s.id = i.service_log_id
WHERE s.vehicle_id = ? AND i.category_id = ?
ORDER BY s.service_date DESC, s.odometer_km DESC, s.id DESC
LIMIT 1;

-- name: CountReminderStatesByVehicle :many
SELECT r.vehicle_id,
       CAST(SUM(r.state = 'due_soon') AS INTEGER) AS due_soon,
       CAST(SUM(r.state = 'overdue') AS INTEGER)  AS overdue
FROM reminders r
JOIN vehicles v ON v.id = r.vehicle_id
WHERE r.paused_at IS NULL AND v.archived_at IS NULL
GROUP BY r.vehicle_id;
