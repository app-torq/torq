-- name: ListRoles :many
SELECT r.*, CAST((SELECT COUNT(*) FROM users u WHERE u.role_id = r.id) AS INTEGER) AS user_count
FROM roles r
ORDER BY CASE r.builtin_key WHEN 'admin' THEN 0 WHEN 'member' THEN 1 WHEN 'viewer' THEN 2 ELSE 3 END,
         r.custom_name COLLATE NOCASE, r.id;

-- name: GetRole :one
SELECT * FROM roles WHERE id = ?;

-- name: GetRoleByBuiltinKey :one
SELECT * FROM roles WHERE builtin_key = ?;

-- name: CreateRole :one
INSERT INTO roles (custom_name, description, all_vehicles, require_mfa, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateRole :one
UPDATE roles SET custom_name = ?, description = ?, all_vehicles = ?, require_mfa = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = ?;

-- name: ListRolePermissions :many
SELECT permission FROM role_permissions WHERE role_id = ? ORDER BY permission;

-- name: ListAllRolePermissions :many
SELECT * FROM role_permissions ORDER BY role_id, permission;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = ?;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission) VALUES (?, ?);

-- name: CountRoleUsers :one
SELECT COUNT(*) FROM users WHERE role_id = ?;

-- name: CountRoleUsersWithoutAccess :one
SELECT COUNT(*) FROM users u
WHERE u.role_id = ? AND NOT EXISTS (SELECT 1 FROM vehicle_access a WHERE a.user_id = u.id);

-- name: MoveRoleUsers :exec
UPDATE users SET role_id = sqlc.arg(to_id), role = sqlc.arg(role), updated_at = sqlc.arg(updated_at)
WHERE role_id = sqlc.arg(from_id);
