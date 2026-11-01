-- +goose Up
CREATE TABLE service_categories (
    id                      INTEGER PRIMARY KEY,
    builtin_key             TEXT UNIQUE,
    custom_name             TEXT,
    is_fluid                INTEGER NOT NULL CHECK (is_fluid IN (0, 1)),
    default_interval_km     INTEGER CHECK (default_interval_km > 0),
    default_interval_months INTEGER CHECK (default_interval_months BETWEEN 1 AND 240),
    sort_order              INTEGER NOT NULL,
    archived_at             TEXT,
    CHECK ((builtin_key IS NULL) <> (custom_name IS NULL))
);

CREATE UNIQUE INDEX service_categories_custom_name ON service_categories (custom_name COLLATE NOCASE) WHERE custom_name IS NOT NULL;

INSERT INTO service_categories (builtin_key, is_fluid, default_interval_km, default_interval_months, sort_order) VALUES
    ('engine_oil',         1, 15000,  12, 1),
    ('transmission_fluid', 1, 60000,  NULL, 2),
    ('brake_fluid',        1, NULL,   24, 3),
    ('coolant',            1, NULL,   48, 4),
    ('differential_fluid', 1, 60000,  NULL, 5),
    ('air_filter',         0, 30000,  24, 6),
    ('cabin_filter',       0, 15000,  12, 7),
    ('fuel_filter',        0, 60000,  NULL, 8),
    ('spark_glow_plugs',   0, 60000,  NULL, 9),
    ('brake_pads_rotors',  0, NULL,   NULL, 10),
    ('timing_belt_chain',  0, 120000, 72, 11);

CREATE TABLE service_logs (
    id               INTEGER PRIMARY KEY,
    vehicle_id       INTEGER NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    service_date     TEXT    NOT NULL,
    odometer_km      INTEGER NOT NULL CHECK (odometer_km >= 0),
    notes            TEXT    NOT NULL DEFAULT '' CHECK (length(notes) <= 10000),
    extra_cost_cents INTEGER NOT NULL DEFAULT 0 CHECK (extra_cost_cents >= 0),
    extra_cost_label TEXT    NOT NULL DEFAULT '',
    created_by       INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at       TEXT    NOT NULL,
    updated_at       TEXT    NOT NULL
);

CREATE INDEX service_logs_vehicle ON service_logs (vehicle_id, service_date DESC, odometer_km DESC);

CREATE TABLE service_log_items (
    id                      INTEGER PRIMARY KEY,
    service_log_id          INTEGER NOT NULL REFERENCES service_logs (id) ON DELETE CASCADE,
    category_id             INTEGER NOT NULL REFERENCES service_categories (id) ON DELETE RESTRICT,
    brand                   TEXT    NOT NULL DEFAULT '',
    specification           TEXT    NOT NULL DEFAULT '',
    quantity_ml             INTEGER CHECK (quantity_ml > 0),
    quantity_unit           TEXT    CHECK (quantity_unit IN ('L', 'mL')),
    oem_part_number         TEXT    NOT NULL DEFAULT '',
    aftermarket_part_number TEXT    NOT NULL DEFAULT '',
    cost_cents              INTEGER CHECK (cost_cents >= 0),
    cost_type               TEXT    NOT NULL CHECK (cost_type IN ('parts', 'fluids')),
    sort_order              INTEGER NOT NULL,
    CHECK ((quantity_ml IS NULL) = (quantity_unit IS NULL)),
    UNIQUE (service_log_id, category_id)
);

CREATE INDEX service_log_items_category ON service_log_items (category_id, service_log_id);

CREATE TABLE reminders (
    id                    INTEGER PRIMARY KEY,
    vehicle_id            INTEGER NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    service_category_id   INTEGER REFERENCES service_categories (id) ON DELETE RESTRICT,
    title                 TEXT    NOT NULL DEFAULT '',
    interval_km           INTEGER CHECK (interval_km > 0),
    interval_months       INTEGER CHECK (interval_months BETWEEN 1 AND 240),
    interval_days         INTEGER CHECK (interval_days BETWEEN 1 AND 3650),
    lead_km               INTEGER CHECK (lead_km >= 0),
    lead_days             INTEGER CHECK (lead_days >= 0),
    start_date            TEXT    NOT NULL,
    start_odometer_km     INTEGER NOT NULL CHECK (start_odometer_km >= 0),
    last_done_date        TEXT,
    last_done_odometer_km INTEGER,
    next_due_km           INTEGER,
    next_due_date         TEXT,
    state                 TEXT    NOT NULL DEFAULT 'upcoming' CHECK (state IN ('upcoming', 'due_soon', 'overdue')),
    state_trigger         TEXT    CHECK (state_trigger IN ('km', 'time')),
    state_changed_at      TEXT    NOT NULL,
    last_notified_state   TEXT    NOT NULL DEFAULT 'upcoming' CHECK (last_notified_state IN ('upcoming', 'due_soon', 'overdue')),
    paused_at             TEXT,
    created_by            INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at            TEXT    NOT NULL,
    updated_at            TEXT    NOT NULL,
    CHECK (interval_km IS NOT NULL OR interval_months IS NOT NULL OR interval_days IS NOT NULL),
    CHECK (NOT (interval_months IS NOT NULL AND interval_days IS NOT NULL)),
    CHECK (service_category_id IS NOT NULL OR length(title) > 0)
);

CREATE INDEX reminders_vehicle_state ON reminders (vehicle_id, state);
CREATE INDEX reminders_category ON reminders (service_category_id);

ALTER TABLE odometer_readings ADD COLUMN service_log_id INTEGER REFERENCES service_logs (id) ON DELETE CASCADE;
