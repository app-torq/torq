-- name: CreateAttachment :one
INSERT INTO attachments (storage_key, original_name, mime_type, size_bytes, purpose, vehicle_id, service_log_id,
                         repair_log_id, spec_entry_id, caption, uploaded_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAttachment :one
SELECT * FROM attachments WHERE id = ?;

-- name: ListAttachmentsForServiceLog :many
SELECT * FROM attachments WHERE service_log_id = ? ORDER BY id;

-- name: ListAttachmentsForRepair :many
SELECT * FROM attachments WHERE repair_log_id = ? ORDER BY id;

-- name: ListAttachmentsForSpecEntry :many
SELECT * FROM attachments WHERE spec_entry_id = ? ORDER BY id;

-- name: GetVehicleAvatar :one
SELECT * FROM attachments WHERE vehicle_id = ? AND purpose = 'avatar';

-- name: ListVehicleAvatars :many
SELECT id, vehicle_id FROM attachments WHERE purpose = 'avatar';

-- name: DeleteAttachment :exec
DELETE FROM attachments WHERE id = ?;

-- name: UpdateAttachmentCaption :one
UPDATE attachments SET caption = ? WHERE id = ? RETURNING *;

-- name: ListStorageKeys :many
SELECT storage_key FROM attachments;

-- name: StorageKeysForServiceLog :many
SELECT storage_key FROM attachments WHERE service_log_id = ?;

-- name: StorageKeysForRepair :many
SELECT storage_key FROM attachments WHERE repair_log_id = ?;

-- name: StorageKeysForSpecEntry :many
SELECT storage_key FROM attachments WHERE spec_entry_id = ?;

-- name: StorageKeysForVehicle :many
SELECT a.storage_key FROM attachments a
WHERE a.vehicle_id = sqlc.arg(vehicle_id)
   OR a.service_log_id IN (SELECT id FROM service_logs WHERE vehicle_id = sqlc.arg(vehicle_id))
   OR a.repair_log_id IN (SELECT id FROM repair_logs WHERE vehicle_id = sqlc.arg(vehicle_id))
   OR a.spec_entry_id IN (SELECT id FROM vehicle_spec_entries WHERE vehicle_id = sqlc.arg(vehicle_id));

-- name: GetAttachmentVehicle :one
-- The vehicle an attachment belongs to, through whatever owns it (0 if none).
SELECT a.vehicle_id AS avatar_vehicle_id, a.service_log_id, a.repair_log_id, a.spec_entry_id,
       CAST(COALESCE(a.vehicle_id, sl.vehicle_id, rl.vehicle_id, se.vehicle_id, 0) AS INTEGER) AS owner_vehicle_id
FROM attachments a
LEFT JOIN service_logs sl ON sl.id = a.service_log_id
LEFT JOIN repair_logs rl ON rl.id = a.repair_log_id
LEFT JOIN vehicle_spec_entries se ON se.id = a.spec_entry_id
WHERE a.id = ?;
