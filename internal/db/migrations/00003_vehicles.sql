-- +goose Up
CREATE TABLE vehicles (
    id                  INTEGER PRIMARY KEY,
    make                TEXT    NOT NULL CHECK (length(make) BETWEEN 1 AND 64),
    model               TEXT    NOT NULL CHECK (length(model) BETWEEN 1 AND 64),
    chassis_code        TEXT    NOT NULL DEFAULT '',
    model_year          INTEGER,
    engine_code         TEXT    NOT NULL DEFAULT '',
    vin                 TEXT    CHECK (vin IS NULL OR length(vin) = 17),
    license_plate       TEXT    CHECK (license_plate IS NULL OR length(license_plate) <= 16),
    current_odometer_km INTEGER NOT NULL CHECK (current_odometer_km >= 0),
    archived_at         TEXT,
    created_by          INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at          TEXT    NOT NULL,
    updated_at          TEXT    NOT NULL
);

-- service_log_id / repair_log_id are added by later migrations once those tables exist.
CREATE TABLE odometer_readings (
    id           INTEGER PRIMARY KEY,
    vehicle_id   INTEGER NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    reading_date TEXT    NOT NULL,
    odometer_km  INTEGER NOT NULL CHECK (odometer_km >= 0),
    source       TEXT    NOT NULL CHECK (source IN ('initial', 'quick_update', 'service_log', 'repair_completion', 'reminder_done')),
    created_at   TEXT    NOT NULL
);

CREATE INDEX odometer_readings_vehicle ON odometer_readings (vehicle_id, reading_date, odometer_km);

CREATE TABLE vehicle_specs (
    vehicle_id                     INTEGER PRIMARY KEY REFERENCES vehicles (id) ON DELETE CASCADE,
    engine_oil_viscosity           TEXT    NOT NULL DEFAULT '',
    engine_oil_spec                TEXT    NOT NULL DEFAULT '',
    engine_oil_capacity_ml         INTEGER CHECK (engine_oil_capacity_ml > 0),
    transmission_fluid_spec        TEXT    NOT NULL DEFAULT '',
    transmission_fluid_capacity_ml INTEGER CHECK (transmission_fluid_capacity_ml > 0),
    differential_fluid_spec        TEXT    NOT NULL DEFAULT '',
    differential_fluid_capacity_ml INTEGER CHECK (differential_fluid_capacity_ml > 0),
    brake_fluid_spec               TEXT    NOT NULL DEFAULT '',
    brake_fluid_capacity_ml        INTEGER CHECK (brake_fluid_capacity_ml > 0),
    coolant_spec                   TEXT    NOT NULL DEFAULT '',
    coolant_capacity_ml            INTEGER CHECK (coolant_capacity_ml > 0),
    wheel_lug_torque_nm            REAL    CHECK (wheel_lug_torque_nm > 0),
    tire_pressure_front_bar        REAL    CHECK (tire_pressure_front_bar > 0 AND tire_pressure_front_bar <= 10),
    tire_pressure_rear_bar         REAL    CHECK (tire_pressure_rear_bar > 0 AND tire_pressure_rear_bar <= 10),
    updated_at                     TEXT    NOT NULL
);

CREATE TABLE vehicle_spec_entries (
    id         INTEGER PRIMARY KEY,
    vehicle_id INTEGER NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    label      TEXT    NOT NULL CHECK (length(label) BETWEEN 1 AND 80),
    value      TEXT    NOT NULL DEFAULT '' CHECK (length(value) <= 2000),
    sort_order INTEGER NOT NULL,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL
);

CREATE INDEX vehicle_spec_entries_vehicle ON vehicle_spec_entries (vehicle_id, sort_order);
