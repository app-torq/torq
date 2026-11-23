-- +goose Up
-- Motorcycles (#28): a vehicle type, motorcycle spec fields, service
-- categories per vehicle type and motorcycles in the catalog. Every existing
-- row becomes a car through the defaults.
ALTER TABLE vehicles ADD COLUMN vehicle_type TEXT NOT NULL DEFAULT 'car' CHECK (vehicle_type IN ('car', 'motorcycle'));

-- Spec texts are length-checked in Go, like the existing *_spec columns.
-- Valve clearances are ranges in mm; the max needs the min (checked in Go).
ALTER TABLE vehicle_specs ADD COLUMN final_drive TEXT CHECK (final_drive IN ('chain', 'belt', 'shaft'));
ALTER TABLE vehicle_specs ADD COLUMN sprocket_front_teeth INTEGER CHECK (sprocket_front_teeth BETWEEN 5 AND 99);
ALTER TABLE vehicle_specs ADD COLUMN sprocket_rear_teeth INTEGER CHECK (sprocket_rear_teeth BETWEEN 10 AND 150);
ALTER TABLE vehicle_specs ADD COLUMN chain_size TEXT NOT NULL DEFAULT '';
ALTER TABLE vehicle_specs ADD COLUMN chain_links INTEGER CHECK (chain_links BETWEEN 20 AND 300);
ALTER TABLE vehicle_specs ADD COLUMN fork_oil_spec TEXT NOT NULL DEFAULT '';
ALTER TABLE vehicle_specs ADD COLUMN fork_oil_capacity_ml INTEGER CHECK (fork_oil_capacity_ml BETWEEN 1 AND 5000);
ALTER TABLE vehicle_specs ADD COLUMN valve_clearance_intake_min_mm REAL CHECK (valve_clearance_intake_min_mm > 0 AND valve_clearance_intake_min_mm <= 2);
ALTER TABLE vehicle_specs ADD COLUMN valve_clearance_intake_max_mm REAL CHECK (valve_clearance_intake_max_mm > 0 AND valve_clearance_intake_max_mm <= 2);
ALTER TABLE vehicle_specs ADD COLUMN valve_clearance_exhaust_min_mm REAL CHECK (valve_clearance_exhaust_min_mm > 0 AND valve_clearance_exhaust_min_mm <= 2);
ALTER TABLE vehicle_specs ADD COLUMN valve_clearance_exhaust_max_mm REAL CHECK (valve_clearance_exhaust_max_mm > 0 AND valve_clearance_exhaust_max_mm <= 2);

-- NULL: the category applies to every vehicle type (always for custom ones).
ALTER TABLE service_categories ADD COLUMN vehicle_type TEXT CHECK (vehicle_type IN ('car', 'motorcycle'));

UPDATE service_categories SET vehicle_type = 'car' WHERE builtin_key IN ('cabin_filter', 'differential_fluid');

INSERT INTO service_categories (builtin_key, is_fluid, default_interval_km, default_interval_months, sort_order, vehicle_type)
SELECT k, f, km, mo, (SELECT MAX(sort_order) FROM service_categories) + n, 'motorcycle'
FROM (SELECT 'chain_maintenance' AS k, 0 AS f, 1000 AS km, NULL AS mo, 1 AS n
      UNION ALL SELECT 'chain_sprockets', 0, NULL, NULL, 2
      UNION ALL SELECT 'valve_clearance', 0, 24000, NULL, 3
      UNION ALL SELECT 'fork_oil', 1, NULL, 24, 4
      UNION ALL SELECT 'final_drive_oil', 1, NULL, 24, 5
      UNION ALL SELECT 'drive_belt', 0, NULL, NULL, 6);

ALTER TABLE catalog_vehicles ADD COLUMN vehicle_type TEXT NOT NULL DEFAULT 'car' CHECK (vehicle_type IN ('car', 'motorcycle'));
CREATE INDEX catalog_vehicles_type ON catalog_vehicles (vehicle_type, make_key, model_key);

-- Force a re-import of the bundled seed so motorcycles arrive.
UPDATE catalog_meta SET seed_sha256 = '', bundled_sha256 = '' WHERE id = 1;
