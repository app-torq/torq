-- +goose Up
-- Tyre pressure as a range: tire_pressure_*_bar stays the single value or the
-- lower bound (normal load), the new *_max_bar the upper bound (full load).
ALTER TABLE vehicle_specs ADD COLUMN tire_pressure_front_max_bar REAL CHECK (tire_pressure_front_max_bar > 0 AND tire_pressure_front_max_bar <= 10);
ALTER TABLE vehicle_specs ADD COLUMN tire_pressure_rear_max_bar REAL CHECK (tire_pressure_rear_max_bar > 0 AND tire_pressure_rear_max_bar <= 10);
