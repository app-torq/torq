-- Public read-only links per vehicle (#38).

-- name: GetVehicleShare :one
SELECT * FROM vehicle_shares WHERE vehicle_id = ?;

-- name: ResolveVehicleShare :one
-- The only lookup by token: by its hash, with what decides whether the link
-- may be shown.
SELECT vehicle_shares.*, vehicles.archived_at AS vehicle_archived_at,
       (SELECT public_links_enabled FROM instance_settings WHERE id = 1) AS links_enabled
FROM vehicle_shares JOIN vehicles ON vehicles.id = vehicle_shares.vehicle_id
WHERE vehicle_shares.token_sha256 = ?;

-- name: InsertVehicleShare :one
INSERT INTO vehicle_shares (vehicle_id, token, token_sha256, include_costs, include_license_plate, include_vin, created_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateVehicleShareOptions :one
UPDATE vehicle_shares SET include_costs = ?, include_license_plate = ?, include_vin = ?
WHERE vehicle_id = ?
RETURNING *;

-- name: RenewVehicleShare :one
UPDATE vehicle_shares SET token = ?, token_sha256 = ?, created_by = ?, created_at = ?, last_opened_at = NULL
WHERE vehicle_id = ?
RETURNING *;

-- name: DeleteVehicleShare :exec
DELETE FROM vehicle_shares WHERE vehicle_id = ?;

-- name: TouchVehicleShare :exec
-- Records a view at most once an hour, so public views rarely write.
UPDATE vehicle_shares SET last_opened_at = sqlc.arg(now)
WHERE vehicle_id = sqlc.arg(vehicle_id) AND (last_opened_at IS NULL OR last_opened_at < sqlc.arg(hour_ago));

-- name: ListVehicleShares :many
SELECT vehicle_shares.vehicle_id, vehicle_shares.created_at, vehicle_shares.last_opened_at,
       vehicles.make, vehicles.model, vehicles.archived_at, users.display_name AS created_by_name
FROM vehicle_shares
JOIN vehicles ON vehicles.id = vehicle_shares.vehicle_id
LEFT JOIN users ON users.id = vehicle_shares.created_by
ORDER BY vehicles.make COLLATE NOCASE, vehicles.model COLLATE NOCASE, vehicle_shares.vehicle_id;
