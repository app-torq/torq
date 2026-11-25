-- name: ListRepairPriorities :many
SELECT p.*, CAST((SELECT COUNT(*) FROM repair_logs r WHERE r.priority_id = p.id
                  AND (CAST(sqlc.narg(scope) AS TEXT) IS NULL OR r.vehicle_id IN (SELECT value FROM json_each(CAST(sqlc.narg(scope) AS TEXT))))) AS INTEGER) AS project_count
FROM repair_priorities p
ORDER BY p.sort_order, p.id;

-- name: GetRepairPriority :one
SELECT * FROM repair_priorities WHERE id = ?;

-- name: GetDefaultRepairPriority :one
SELECT * FROM repair_priorities WHERE is_default = 1;

-- name: CreateRepairPriority :one
INSERT INTO repair_priorities (custom_name, color, sort_order, created_at, updated_at)
VALUES (?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM repair_priorities), ?, ?)
RETURNING *;

-- name: UpdateRepairPriority :one
UPDATE repair_priorities SET custom_name = ?, color = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: SetRepairPriorityOrder :exec
UPDATE repair_priorities SET sort_order = ? WHERE id = ?;

-- name: ClearDefaultRepairPriority :exec
UPDATE repair_priorities SET is_default = 0 WHERE is_default = 1;

-- name: SetDefaultRepairPriority :exec
UPDATE repair_priorities SET is_default = 1 WHERE id = ?;

-- name: MoveRepairsToPriority :exec
UPDATE repair_logs SET priority_id = sqlc.narg(to_id) WHERE priority_id = sqlc.arg(from_id);

-- name: DeleteRepairPriority :exec
DELETE FROM repair_priorities WHERE id = ?;
