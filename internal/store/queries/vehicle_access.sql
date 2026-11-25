-- name: ListUserVehicleAccess :many
SELECT * FROM vehicle_access WHERE user_id = ? ORDER BY vehicle_id;

-- name: DeleteUserVehicleAccess :exec
DELETE FROM vehicle_access WHERE user_id = ?;

-- name: AddVehicleAccess :exec
INSERT INTO vehicle_access (user_id, vehicle_id, level, created_at) VALUES (?, ?, ?, ?)
ON CONFLICT (user_id, vehicle_id) DO UPDATE SET level = excluded.level;

-- name: CountVehicleAccessByUser :many
SELECT user_id, COUNT(*) AS vehicle_count FROM vehicle_access GROUP BY user_id;

-- name: ListVehicleAccessUsers :many
-- Active users who see the vehicle: through a role that sees all vehicles,
-- or through an assignment while their role uses assigned vehicles.
SELECT u.id, u.username, u.display_name, u.role_id, r.builtin_key, r.custom_name,
       CAST(CASE WHEN r.all_vehicles = 1 THEN 'role' ELSE 'assignment' END AS TEXT) AS via,
       CAST(CASE WHEN r.all_vehicles = 1 THEN 'edit' ELSE a.level END AS TEXT) AS level
FROM users u
JOIN roles r ON r.id = u.role_id
LEFT JOIN vehicle_access a ON a.user_id = u.id AND a.vehicle_id = sqlc.arg(vehicle_id)
WHERE u.disabled_at IS NULL AND (r.all_vehicles = 1 OR a.vehicle_id IS NOT NULL)
ORDER BY u.username COLLATE NOCASE;
