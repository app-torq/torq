-- name: ListRepairTags :many
SELECT t.*, CAST((SELECT COUNT(*) FROM repair_log_tags rt WHERE rt.tag_id = t.id) AS INTEGER) AS project_count
FROM repair_tags t
ORDER BY t.name COLLATE NOCASE, t.id;

-- name: AllRepairTags :many
SELECT * FROM repair_tags ORDER BY id;

-- name: GetRepairTag :one
SELECT * FROM repair_tags WHERE id = ?;

-- name: CreateRepairTag :one
INSERT INTO repair_tags (name, created_at, updated_at) VALUES (?, ?, ?) RETURNING *;

-- name: RenameRepairTag :one
UPDATE repair_tags SET name = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: DeleteRepairTag :exec
DELETE FROM repair_tags WHERE id = ?;

-- name: DeleteRepairLogTags :exec
DELETE FROM repair_log_tags WHERE repair_log_id = ?;

-- name: InsertRepairLogTag :exec
INSERT INTO repair_log_tags (repair_log_id, tag_id) VALUES (?, ?);

-- name: ListRepairLogTags :many
SELECT rt.repair_log_id, t.name FROM repair_log_tags rt
JOIN repair_tags t ON t.id = rt.tag_id
WHERE rt.repair_log_id IN (sqlc.slice(ids))
ORDER BY rt.repair_log_id, t.name COLLATE NOCASE, t.id;
