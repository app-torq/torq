-- name: CreateWebhook :one
INSERT INTO webhooks (name, url, preset, enabled, on_due_soon, on_overdue, on_weekly_digest, on_monthly_digest,
                      owner_user_id, last_weekly_digest_on, last_monthly_digest_on, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = ?;

-- name: ListWebhooks :many
SELECT * FROM webhooks ORDER BY name COLLATE NOCASE, id;

-- name: UpdateWebhook :one
UPDATE webhooks
SET name = ?, url = ?, preset = ?, enabled = ?, on_due_soon = ?, on_overdue = ?, on_weekly_digest = ?,
    on_monthly_digest = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = ?;

-- name: ListWebhookHeaders :many
SELECT * FROM webhook_headers WHERE webhook_id = ? ORDER BY id;

-- name: UpsertWebhookHeader :exec
INSERT INTO webhook_headers (webhook_id, name, value) VALUES (?, ?, ?)
ON CONFLICT (webhook_id, name COLLATE NOCASE) DO UPDATE SET value = excluded.value, name = excluded.name;

-- name: DeleteWebhookHeader :exec
DELETE FROM webhook_headers WHERE id = ?;

-- name: ListSubscribedWebhooks :many
SELECT * FROM webhooks
WHERE enabled = 1
  AND CASE CAST(sqlc.arg(event) AS TEXT)
          WHEN 'reminder_due_soon' THEN on_due_soon
          WHEN 'reminder_overdue' THEN on_overdue
          WHEN 'digest_weekly' THEN on_weekly_digest
          WHEN 'digest_monthly' THEN on_monthly_digest
          ELSE 0 END = 1
ORDER BY id;

-- name: InsertDelivery :one
INSERT INTO webhook_deliveries (webhook_id, event, reminder_id, request_body, request_headers_json, status,
                                attempt_count, next_attempt_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: ClaimDueDeliveries :many
SELECT * FROM webhook_deliveries
WHERE status = 'pending' AND next_attempt_at <= ?
ORDER BY next_attempt_at, id
LIMIT 20;

-- name: RecordAttempt :exec
UPDATE webhook_deliveries
SET status = ?, attempt_count = ?, next_attempt_at = ?, last_status_code = ?, last_error = ?,
    response_excerpt = ?, duration_ms = ?, completed_at = ?
WHERE id = ?;

-- name: ListDeliveries :many
SELECT * FROM webhook_deliveries WHERE webhook_id = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?;

-- name: CountDeliveries :one
SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = ?;

-- name: PruneDeliveries :execrows
DELETE FROM webhook_deliveries
WHERE status <> 'pending'
  AND id NOT IN (SELECT d.id FROM webhook_deliveries d
                 WHERE d.webhook_id = webhook_deliveries.webhook_id AND d.status <> 'pending'
                 ORDER BY d.created_at DESC, d.id DESC LIMIT 100);

-- name: SetWeeklyDigestSent :exec
UPDATE webhooks SET last_weekly_digest_on = ? WHERE id = ?;

-- name: SetMonthlyDigestSent :exec
UPDATE webhooks SET last_monthly_digest_on = ? WHERE id = ?;

-- name: ListDueReminders :many
SELECT r.*, v.make, v.model, v.license_plate, v.current_odometer_km
FROM reminders r
JOIN vehicles v ON v.id = r.vehicle_id
WHERE r.state IN ('due_soon', 'overdue') AND r.paused_at IS NULL AND v.archived_at IS NULL
ORDER BY CASE r.state WHEN 'overdue' THEN 0 ELSE 1 END, r.id;

-- name: GetReminderWithVehicle :one
SELECT r.*, v.make, v.model, v.license_plate, v.current_odometer_km
FROM reminders r
JOIN vehicles v ON v.id = r.vehicle_id
WHERE r.id = ?;

-- name: GetDelivery :one
SELECT * FROM webhook_deliveries WHERE id = ?;
