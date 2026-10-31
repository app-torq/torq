-- name: ListCategories :many
SELECT * FROM service_categories
WHERE CAST(sqlc.arg(include_archived) AS INTEGER) = 1 OR archived_at IS NULL
ORDER BY builtin_key IS NULL, sort_order, custom_name COLLATE NOCASE;

-- name: GetCategory :one
SELECT * FROM service_categories WHERE id = ?;

-- name: CreateCustomCategory :one
INSERT INTO service_categories (custom_name, is_fluid, default_interval_km, default_interval_months, sort_order)
VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM service_categories))
RETURNING *;

-- name: UpdateCategory :one
UPDATE service_categories
SET custom_name = ?, is_fluid = ?, default_interval_km = ?, default_interval_months = ?
WHERE id = ?
RETURNING *;

-- name: SetCategoryArchived :one
UPDATE service_categories SET archived_at = ? WHERE id = ? RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM service_categories WHERE id = ?;

-- name: CategoryInUse :one
SELECT CAST(EXISTS (SELECT 1 FROM service_log_items WHERE category_id = sqlc.arg(id))
         OR EXISTS (SELECT 1 FROM reminders WHERE service_category_id = sqlc.arg(id)) AS INTEGER);
