-- name: ServiceCostsByType :many
SELECT sl.vehicle_id, i.cost_type, CAST(COALESCE(SUM(i.cost_cents), 0) AS INTEGER) AS cents
FROM service_log_items i
JOIN service_logs sl ON sl.id = i.service_log_id
WHERE (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR sl.vehicle_id = sqlc.arg(vehicle_id))
  AND sl.service_date >= sqlc.arg(from_date) AND sl.service_date <= sqlc.arg(to_date)
GROUP BY sl.vehicle_id, i.cost_type;

-- name: ServiceExtraCosts :many
SELECT vehicle_id, CAST(COALESCE(SUM(extra_cost_cents), 0) AS INTEGER) AS cents
FROM service_logs
WHERE (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR vehicle_id = sqlc.arg(vehicle_id))
  AND service_date >= sqlc.arg(from_date) AND service_date <= sqlc.arg(to_date)
GROUP BY vehicle_id;

-- name: RepairCostsByType :many
SELECT r.vehicle_id, p.cost_type, CAST(COALESCE(SUM(p.quantity * p.unit_cost_cents), 0) AS INTEGER) AS cents
FROM repair_parts p
JOIN repair_logs r ON r.id = p.repair_log_id
WHERE r.status = 'completed'
  AND (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR r.vehicle_id = sqlc.arg(vehicle_id))
  AND r.completed_on >= sqlc.arg(from_date) AND r.completed_on <= sqlc.arg(to_date)
GROUP BY r.vehicle_id, p.cost_type;

-- name: ReadingRange :many
SELECT vehicle_id,
       CAST(MIN(odometer_km) AS INTEGER) AS min_km,
       CAST(MAX(odometer_km) AS INTEGER) AS max_km,
       COUNT(*) AS readings
FROM odometer_readings
WHERE (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR vehicle_id = sqlc.arg(vehicle_id))
  AND reading_date >= sqlc.arg(from_date) AND reading_date <= sqlc.arg(to_date)
GROUP BY vehicle_id;

-- name: HourReadingRange :many
-- Hours span of vehicles tracked by hours only (cost per operating hour).
SELECT m.vehicle_id,
       CAST(MIN(hr.hours_tenths) AS INTEGER) AS min_tenths,
       CAST(MAX(hr.hours_tenths) AS INTEGER) AS max_tenths,
       COUNT(*) AS readings
FROM hour_meter_readings hr
JOIN hour_meters m ON m.id = hr.hour_meter_id
JOIN vehicles v ON v.id = m.vehicle_id
WHERE v.has_odometer = 0
  AND (CAST(sqlc.arg(vehicle_id) AS INTEGER) = 0 OR m.vehicle_id = sqlc.arg(vehicle_id))
  AND hr.reading_date >= sqlc.arg(from_date) AND hr.reading_date <= sqlc.arg(to_date)
GROUP BY m.vehicle_id;

-- name: TimelineServiceLogs :many
SELECT * FROM service_logs
WHERE vehicle_id = sqlc.arg(vehicle_id)
  AND service_date >= sqlc.arg(from_date) AND service_date <= sqlc.arg(to_date)
ORDER BY service_date DESC, odometer_km DESC, id DESC;

-- name: TimelineRepairs :many
SELECT * FROM repair_logs
WHERE vehicle_id = sqlc.arg(vehicle_id) AND status = 'completed'
  AND completed_on >= sqlc.arg(from_date) AND completed_on <= sqlc.arg(to_date)
ORDER BY completed_on DESC, completion_odometer_km DESC, id DESC;
