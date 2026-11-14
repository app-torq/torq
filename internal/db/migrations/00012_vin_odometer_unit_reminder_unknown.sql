-- +goose NO TRANSACTION
-- +goose Up
-- VINs of any length up to 32 characters (JDM chassis numbers, pre-1981
-- VINs), the vehicle's odometer unit, power in watts so hp and PS survive a
-- round trip, and reminders whose last service is unknown.
--
-- SQLite cannot change a CHECK constraint, so vehicles is rebuilt following
-- https://sqlite.org/lang_altertable.html#otheralter. Foreign keys must be
-- off for that, and the pragma has no effect inside a transaction.
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE vehicles_new (
    id                  INTEGER PRIMARY KEY,
    make                TEXT    NOT NULL CHECK (length(make) BETWEEN 1 AND 64),
    model               TEXT    NOT NULL CHECK (length(model) BETWEEN 1 AND 64),
    chassis_code        TEXT    NOT NULL DEFAULT '',
    model_year          INTEGER,
    engine_code         TEXT    NOT NULL DEFAULT '',
    vin                 TEXT    CHECK (vin IS NULL OR length(vin) BETWEEN 1 AND 32),
    license_plate       TEXT    CHECK (license_plate IS NULL OR length(license_plate) <= 16),
    current_odometer_km INTEGER NOT NULL CHECK (current_odometer_km >= 0),
    archived_at         TEXT,
    created_by          INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at          TEXT    NOT NULL,
    updated_at          TEXT    NOT NULL,
    hsn                 TEXT    CHECK (hsn IS NULL OR (length(hsn) = 4 AND hsn NOT GLOB '*[^0-9]*')),
    tsn                 TEXT    CHECK (tsn IS NULL OR (length(tsn) = 3 AND tsn NOT GLOB '*[^0-9A-Z]*')),
    displacement_cc     INTEGER CHECK (displacement_cc IS NULL OR displacement_cc BETWEEN 1 AND 20000),
    power_kw            INTEGER CHECK (power_kw IS NULL OR power_kw BETWEEN 1 AND 2000),
    fuel_type           TEXT    CHECK (fuel_type IS NULL OR fuel_type IN ('diesel', 'petrol', 'hybrid', 'electric', 'lpg', 'cng', 'other')),
    odometer_unit       TEXT    NOT NULL DEFAULT 'km' CHECK (odometer_unit IN ('km', 'mi')),
    power_w             INTEGER CHECK (power_w IS NULL OR power_w BETWEEN 1 AND 2000000)
);

INSERT INTO vehicles_new (id, make, model, chassis_code, model_year, engine_code, vin, license_plate,
                          current_odometer_km, archived_at, created_by, created_at, updated_at,
                          hsn, tsn, displacement_cc, power_kw, fuel_type, power_w)
SELECT id, make, model, chassis_code, model_year, engine_code, vin, license_plate,
       current_odometer_km, archived_at, created_by, created_at, updated_at,
       hsn, tsn, displacement_cc, power_kw, fuel_type, power_kw * 1000
FROM vehicles;

DROP TABLE vehicles;

ALTER TABLE vehicles_new RENAME TO vehicles;

ALTER TABLE reminders ADD COLUMN last_done_unknown INTEGER NOT NULL DEFAULT 0 CHECK (last_done_unknown IN (0, 1));

COMMIT;

PRAGMA foreign_keys = ON;
