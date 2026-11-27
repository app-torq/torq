-- name: ListUserVehicleAccess :many
SELECT a.user_id, a.vehicle_id, a.level, a.created_at, a.origin, a.granted_by,
       g.username AS granter_username, g.display_name AS granter_display_name
FROM vehicle_access a
LEFT JOIN users g ON g.id = a.granted_by
WHERE a.user_id = ? ORDER BY a.vehicle_id;

-- name: GetVehicleAccess :one
SELECT * FROM vehicle_access WHERE user_id = ? AND vehicle_id = ?;

-- name: DeleteVehicleAccess :exec
DELETE FROM vehicle_access WHERE user_id = ? AND vehicle_id = ?;

-- name: AddVehicleAccess :exec
INSERT INTO vehicle_access (user_id, vehicle_id, level, created_at, origin, granted_by) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (user_id, vehicle_id) DO UPDATE SET level = excluded.level, origin = excluded.origin, granted_by = excluded.granted_by;

-- name: SetVehicleAccessLevel :exec
-- An administrator's change makes the assignment theirs.
UPDATE vehicle_access SET level = ?, origin = 'admin', granted_by = NULL WHERE user_id = ? AND vehicle_id = ?;

-- name: CountVehicleAccessByUser :many
SELECT user_id, COUNT(*) AS vehicle_count FROM vehicle_access GROUP BY user_id;

-- name: ListSharesBy :many
-- Shares a user made on a vehicle.
SELECT a.user_id, a.level, a.created_at, u.username, u.display_name
FROM vehicle_access a
JOIN users u ON u.id = a.user_id
WHERE a.origin = 'shared' AND a.granted_by = ? AND a.vehicle_id = ?
ORDER BY u.username COLLATE NOCASE;

-- name: ListAllShares :many
SELECT user_id, vehicle_id, granted_by FROM vehicle_access WHERE origin = 'shared' ORDER BY granted_by;

-- name: GetVehicleSharer :one
-- Who shared the vehicle with the user, if it came through a share.
SELECT g.display_name
FROM vehicle_access a
JOIN users g ON g.id = a.granted_by
WHERE a.user_id = ? AND a.vehicle_id = ? AND a.origin = 'shared';

-- name: ListVehicleAccessUsers :many
-- Active users who see the vehicle: through a role that sees all vehicles,
-- or through an assignment while their role uses assigned vehicles.
SELECT u.id, u.username, u.display_name, u.role_id, r.builtin_key, r.custom_name,
       CAST(CASE WHEN r.all_vehicles = 1 THEN 'role' ELSE 'assignment' END AS TEXT) AS via,
       CAST(CASE WHEN r.all_vehicles = 1 THEN 'edit' ELSE a.level END AS TEXT) AS level,
       CAST(CASE WHEN r.all_vehicles = 1 THEN '' ELSE a.origin END AS TEXT) AS origin,
       a.granted_by, g.username AS granter_username, g.display_name AS granter_display_name
FROM users u
JOIN roles r ON r.id = u.role_id
LEFT JOIN vehicle_access a ON a.user_id = u.id AND a.vehicle_id = sqlc.arg(vehicle_id)
LEFT JOIN users g ON g.id = a.granted_by AND r.all_vehicles = 0
WHERE u.disabled_at IS NULL AND (r.all_vehicles = 1 OR a.vehicle_id IS NOT NULL)
ORDER BY u.username COLLATE NOCASE;
