-- name: CreateServiceLog :one
INSERT INTO service_logs (vehicle_id, service_date, odometer_km, notes, extra_cost_cents, extra_cost_label,
                          created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetServiceLog :one
SELECT * FROM service_logs WHERE id = ?;

-- name: ListServiceLogs :many
SELECT sl.* FROM service_logs sl
WHERE (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR sl.vehicle_id = sqlc.arg(vehicle_id))
  AND (CAST(sqlc.arg(category_id) AS INTEGER) = 0
       OR EXISTS (SELECT 1 FROM service_log_items i WHERE i.service_log_id = sl.id AND i.category_id = sqlc.arg(category_id)))
  AND (CAST(sqlc.arg(from_date) AS TEXT) = '' OR sl.service_date >= sqlc.arg(from_date))
  AND (CAST(sqlc.arg(to_date) AS TEXT) = '' OR sl.service_date <= sqlc.arg(to_date))
  AND (CAST(sqlc.arg(q) AS TEXT) = ''
       OR sl.notes LIKE '%' || sqlc.arg(q) || '%'
       OR EXISTS (SELECT 1 FROM service_log_items i WHERE i.service_log_id = sl.id
                  AND (i.brand LIKE '%' || sqlc.arg(q) || '%' OR i.specification LIKE '%' || sqlc.arg(q) || '%'
                       OR i.oem_part_number LIKE '%' || sqlc.arg(q) || '%' OR i.aftermarket_part_number LIKE '%' || sqlc.arg(q) || '%')))
ORDER BY sl.service_date DESC, sl.odometer_km DESC, sl.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountServiceLogs :one
SELECT COUNT(*) FROM service_logs sl
WHERE (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR sl.vehicle_id = sqlc.arg(vehicle_id))
  AND (CAST(sqlc.arg(category_id) AS INTEGER) = 0
       OR EXISTS (SELECT 1 FROM service_log_items i WHERE i.service_log_id = sl.id AND i.category_id = sqlc.arg(category_id)))
  AND (CAST(sqlc.arg(from_date) AS TEXT) = '' OR sl.service_date >= sqlc.arg(from_date))
  AND (CAST(sqlc.arg(to_date) AS TEXT) = '' OR sl.service_date <= sqlc.arg(to_date))
  AND (CAST(sqlc.arg(q) AS TEXT) = ''
       OR sl.notes LIKE '%' || sqlc.arg(q) || '%'
       OR EXISTS (SELECT 1 FROM service_log_items i WHERE i.service_log_id = sl.id
                  AND (i.brand LIKE '%' || sqlc.arg(q) || '%' OR i.specification LIKE '%' || sqlc.arg(q) || '%'
                       OR i.oem_part_number LIKE '%' || sqlc.arg(q) || '%' OR i.aftermarket_part_number LIKE '%' || sqlc.arg(q) || '%')));

-- name: UpdateServiceLog :one
UPDATE service_logs
SET vehicle_id = ?, service_date = ?, odometer_km = ?, notes = ?, extra_cost_cents = ?, extra_cost_label = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteServiceLog :exec
DELETE FROM service_logs WHERE id = ?;

-- name: InsertServiceLogItem :exec
INSERT INTO service_log_items (service_log_id, category_id, brand, specification, quantity_ml, quantity_unit,
                               oem_part_number, aftermarket_part_number, cost_cents, cost_type, sort_order)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteServiceLogItems :exec
DELETE FROM service_log_items WHERE service_log_id = ?;

-- name: ListServiceLogItems :many
SELECT * FROM service_log_items WHERE service_log_id = ? ORDER BY sort_order, id;

-- name: ListServiceLogItemsForLogs :many
SELECT * FROM service_log_items WHERE service_log_id IN (sqlc.slice(ids)) ORDER BY service_log_id, sort_order, id;

-- name: CountInconsistentServiceLogs :one
SELECT COUNT(*) FROM service_logs
WHERE vehicle_id = sqlc.arg(vehicle_id) AND id <> sqlc.arg(id)
  AND ((service_date < sqlc.arg(service_date) AND odometer_km > sqlc.arg(odometer_km))
    OR (service_date > sqlc.arg(service_date) AND odometer_km < sqlc.arg(odometer_km)));

-- name: DeleteServiceLogReading :exec
DELETE FROM odometer_readings WHERE service_log_id = ?;

-- name: LinkReadingToServiceLog :exec
UPDATE odometer_readings SET service_log_id = ? WHERE id = ?;
