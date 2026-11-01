-- name: CreateRepair :one
INSERT INTO repair_logs (vehicle_id, title, status, symptom, root_cause, procedure_notes, special_tools,
                         started_on, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetRepair :one
SELECT * FROM repair_logs WHERE id = ?;

-- name: ListRepairs :many
SELECT r.* FROM repair_logs r
WHERE (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR r.vehicle_id = sqlc.arg(vehicle_id))
  AND instr(',' || CAST(sqlc.arg(statuses) AS TEXT) || ',', ',' || r.status || ',') > 0
  AND (CAST(sqlc.arg(q) AS TEXT) = ''
       OR r.title LIKE '%' || sqlc.arg(q) || '%'
       OR r.symptom LIKE '%' || sqlc.arg(q) || '%'
       OR r.root_cause LIKE '%' || sqlc.arg(q) || '%'
       OR EXISTS (SELECT 1 FROM repair_fault_codes f WHERE f.repair_log_id = r.id AND f.code LIKE '%' || sqlc.arg(q) || '%')
       OR EXISTS (SELECT 1 FROM repair_parts p WHERE p.repair_log_id = r.id
                  AND (p.name LIKE '%' || sqlc.arg(q) || '%' OR p.oem_part_number LIKE '%' || sqlc.arg(q) || '%'
                       OR p.aftermarket_part_number LIKE '%' || sqlc.arg(q) || '%')))
ORDER BY CASE r.status WHEN 'in_progress' THEN 0 WHEN 'waiting_on_parts' THEN 1 WHEN 'planned' THEN 2 ELSE 3 END,
         COALESCE(r.completed_on, r.updated_at) DESC, r.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountRepairs :one
SELECT COUNT(*) FROM repair_logs r
WHERE (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR r.vehicle_id = sqlc.arg(vehicle_id))
  AND instr(',' || CAST(sqlc.arg(statuses) AS TEXT) || ',', ',' || r.status || ',') > 0
  AND (CAST(sqlc.arg(q) AS TEXT) = ''
       OR r.title LIKE '%' || sqlc.arg(q) || '%'
       OR r.symptom LIKE '%' || sqlc.arg(q) || '%'
       OR r.root_cause LIKE '%' || sqlc.arg(q) || '%'
       OR EXISTS (SELECT 1 FROM repair_fault_codes f WHERE f.repair_log_id = r.id AND f.code LIKE '%' || sqlc.arg(q) || '%')
       OR EXISTS (SELECT 1 FROM repair_parts p WHERE p.repair_log_id = r.id
                  AND (p.name LIKE '%' || sqlc.arg(q) || '%' OR p.oem_part_number LIKE '%' || sqlc.arg(q) || '%'
                       OR p.aftermarket_part_number LIKE '%' || sqlc.arg(q) || '%')));

-- name: UpdateRepair :one
UPDATE repair_logs
SET title = ?, symptom = ?, root_cause = ?, procedure_notes = ?, special_tools = ?, started_on = ?,
    completed_on = ?, completion_odometer_km = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: SetRepairStatus :one
UPDATE repair_logs
SET status = ?, started_on = ?, completed_on = ?, completion_odometer_km = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteRepair :exec
DELETE FROM repair_logs WHERE id = ?;

-- name: DeleteFaultCodes :exec
DELETE FROM repair_fault_codes WHERE repair_log_id = ?;

-- name: InsertFaultCode :exec
INSERT INTO repair_fault_codes (repair_log_id, code) VALUES (?, ?);

-- name: ListFaultCodes :many
SELECT * FROM repair_fault_codes WHERE repair_log_id IN (sqlc.slice(ids)) ORDER BY repair_log_id, id;

-- name: DeleteTorqueSpecs :exec
DELETE FROM repair_torque_specs WHERE repair_log_id = ?;

-- name: InsertTorqueSpec :exec
INSERT INTO repair_torque_specs (repair_log_id, label, value_nm, note, sort_order) VALUES (?, ?, ?, ?, ?);

-- name: ListTorqueSpecs :many
SELECT * FROM repair_torque_specs WHERE repair_log_id = ? ORDER BY sort_order, id;

-- name: CreatePart :one
INSERT INTO repair_parts (repair_log_id, name, oem_part_number, aftermarket_brand, aftermarket_part_number,
                          supplier_url, quantity, unit_cost_cents, cost_type, status, sort_order)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(rp.sort_order), 0) + 1 FROM repair_parts rp WHERE rp.repair_log_id = sqlc.arg(parent_id)))
RETURNING *;

-- name: GetPart :one
SELECT * FROM repair_parts WHERE id = ?;

-- name: UpdatePart :one
UPDATE repair_parts
SET name = ?, oem_part_number = ?, aftermarket_brand = ?, aftermarket_part_number = ?, supplier_url = ?,
    quantity = ?, unit_cost_cents = ?, cost_type = ?, status = ?
WHERE id = ?
RETURNING *;

-- name: DeletePart :exec
DELETE FROM repair_parts WHERE id = ?;

-- name: ListParts :many
SELECT * FROM repair_parts WHERE repair_log_id = ? ORDER BY sort_order, id;

-- name: SetPartOrder :exec
UPDATE repair_parts SET sort_order = ? WHERE id = ? AND repair_log_id = ?;

-- name: RepairTotals :many
SELECT repair_log_id,
       CAST(COALESCE(SUM(quantity * unit_cost_cents), 0) AS INTEGER) AS total_cents,
       CAST(SUM(status = 'needed') AS INTEGER)    AS needed,
       CAST(SUM(status = 'ordered') AS INTEGER)   AS ordered,
       CAST(SUM(status = 'in_stock') AS INTEGER)  AS in_stock,
       CAST(SUM(status = 'installed') AS INTEGER) AS installed
FROM repair_parts
WHERE repair_log_id IN (sqlc.slice(ids))
GROUP BY repair_log_id;

-- name: DeleteRepairReading :exec
DELETE FROM odometer_readings WHERE repair_log_id = ?;

-- name: LinkReadingToRepair :exec
UPDATE odometer_readings SET repair_log_id = ? WHERE id = ?;
