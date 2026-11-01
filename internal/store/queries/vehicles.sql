-- name: CreateVehicle :one
INSERT INTO vehicles (make, model, chassis_code, model_year, engine_code, vin, license_plate,
                      hsn, tsn, displacement_cc, power_kw, fuel_type,
                      current_odometer_km, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetVehicle :one
SELECT * FROM vehicles WHERE id = ?;

-- name: ListVehicles :many
-- mode: 0 = active only, 1 = archived only, 2 = all
SELECT * FROM vehicles
WHERE CASE CAST(sqlc.arg(mode) AS INTEGER)
          WHEN 0 THEN archived_at IS NULL
          WHEN 1 THEN archived_at IS NOT NULL
          ELSE 1 END
ORDER BY archived_at IS NOT NULL, make COLLATE NOCASE, model COLLATE NOCASE, id;

-- name: UpdateVehicleProfile :one
UPDATE vehicles
SET make = ?, model = ?, chassis_code = ?, model_year = ?, engine_code = ?, vin = ?, license_plate = ?,
    hsn = ?, tsn = ?, displacement_cc = ?, power_kw = ?, fuel_type = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: SetVehicleArchived :one
UPDATE vehicles SET archived_at = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: DeleteVehicle :exec
DELETE FROM vehicles WHERE id = ?;

-- name: SetCurrentOdometer :exec
UPDATE vehicles SET current_odometer_km = ?, updated_at = ? WHERE id = ?;

-- name: InsertOdometerReading :one
INSERT INTO odometer_readings (vehicle_id, reading_date, odometer_km, source, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id;

-- name: ListOdometerReadings :many
SELECT id, vehicle_id, reading_date, odometer_km, source, created_at FROM odometer_readings
WHERE vehicle_id = sqlc.arg(vehicle_id)
  AND (sqlc.narg(from_date) IS NULL OR reading_date >= sqlc.narg(from_date))
  AND (sqlc.narg(to_date) IS NULL OR reading_date <= sqlc.narg(to_date))
ORDER BY reading_date DESC, odometer_km DESC, id DESC
LIMIT 500;

-- name: CreateVehicleSpecs :exec
INSERT INTO vehicle_specs (vehicle_id, updated_at) VALUES (?, ?);

-- name: GetVehicleSpecs :one
SELECT * FROM vehicle_specs WHERE vehicle_id = ?;

-- name: UpdateVehicleSpecs :one
UPDATE vehicle_specs
SET engine_oil_viscosity = ?, engine_oil_spec = ?, engine_oil_capacity_ml = ?,
    transmission_fluid_spec = ?, transmission_fluid_capacity_ml = ?,
    differential_fluid_spec = ?, differential_fluid_capacity_ml = ?,
    brake_fluid_spec = ?, brake_fluid_capacity_ml = ?,
    coolant_spec = ?, coolant_capacity_ml = ?,
    wheel_lug_torque_nm = ?, tire_pressure_front_bar = ?, tire_pressure_rear_bar = ?, specs_meta = ?,
    updated_at = ?
WHERE vehicle_id = ?
RETURNING *;

-- name: ListSpecEntries :many
SELECT * FROM vehicle_spec_entries WHERE vehicle_id = ? ORDER BY sort_order, id;

-- name: GetSpecEntry :one
SELECT * FROM vehicle_spec_entries WHERE id = ?;

-- name: CreateSpecEntry :one
INSERT INTO vehicle_spec_entries (vehicle_id, label, value, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateSpecEntry :one
UPDATE vehicle_spec_entries SET label = ?, value = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: DeleteSpecEntry :exec
DELETE FROM vehicle_spec_entries WHERE id = ?;

-- name: SetSpecEntryOrder :exec
UPDATE vehicle_spec_entries SET sort_order = ? WHERE id = ? AND vehicle_id = ?;

-- name: MaxSpecEntryOrder :one
SELECT CAST(COALESCE(MAX(sort_order), 0) AS INTEGER) FROM vehicle_spec_entries WHERE vehicle_id = ?;
