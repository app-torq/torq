-- +goose NO TRANSACTION
-- +goose Up
-- Hour meters (#24): a vehicle can track operating hours next to its
-- odometer or instead of it. Hours are stored in tenths (1,250.4 h = 12504).
-- One meter per vehicle for now; dropping hour_meters_one_per_vehicle allows
-- several later without changing any data.
--
-- reminders gains an hours interval and its CHECK constraints change, so it is
-- rebuilt as in 00012 (foreign keys off, outside the transaction).
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE hour_meters (
    id                   INTEGER PRIMARY KEY,
    vehicle_id           INTEGER NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    name                 TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 40),
    current_hours_tenths INTEGER NOT NULL CHECK (current_hours_tenths BETWEEN 0 AND 9999999),
    created_at           TEXT    NOT NULL,
    updated_at           TEXT    NOT NULL
);

CREATE UNIQUE INDEX hour_meters_one_per_vehicle ON hour_meters (vehicle_id);

CREATE TABLE hour_meter_readings (
    id             INTEGER PRIMARY KEY,
    hour_meter_id  INTEGER NOT NULL REFERENCES hour_meters (id) ON DELETE CASCADE,
    reading_date   TEXT    NOT NULL,
    hours_tenths   INTEGER NOT NULL CHECK (hours_tenths BETWEEN 0 AND 9999999),
    source         TEXT    NOT NULL CHECK (source IN ('initial', 'quick_update', 'service_log', 'repair_completion', 'reminder_done')),
    service_log_id INTEGER REFERENCES service_logs (id) ON DELETE CASCADE,
    repair_log_id  INTEGER REFERENCES repair_logs (id) ON DELETE CASCADE,
    created_at     TEXT    NOT NULL
);

CREATE INDEX hour_meter_readings_meter ON hour_meter_readings (hour_meter_id, reading_date, hours_tenths);
CREATE UNIQUE INDEX hour_meter_readings_service_log ON hour_meter_readings (service_log_id, hour_meter_id) WHERE service_log_id IS NOT NULL;
CREATE UNIQUE INDEX hour_meter_readings_repair ON hour_meter_readings (repair_log_id, hour_meter_id) WHERE repair_log_id IS NOT NULL;

-- Without an odometer, current_odometer_km stays 0 and means nothing; entries
-- then store 0 km with *_recorded = 0.
ALTER TABLE vehicles ADD COLUMN has_odometer INTEGER NOT NULL DEFAULT 1 CHECK (has_odometer IN (0, 1));
ALTER TABLE service_logs ADD COLUMN odometer_recorded INTEGER NOT NULL DEFAULT 1 CHECK (odometer_recorded IN (0, 1));
ALTER TABLE repair_logs ADD COLUMN completion_odometer_recorded INTEGER NOT NULL DEFAULT 1 CHECK (completion_odometer_recorded IN (0, 1));

CREATE TABLE reminders_new (
    id                     INTEGER PRIMARY KEY,
    vehicle_id             INTEGER NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    service_category_id    INTEGER REFERENCES service_categories (id) ON DELETE RESTRICT,
    title                  TEXT    NOT NULL DEFAULT '',
    interval_km            INTEGER CHECK (interval_km > 0),
    interval_months        INTEGER CHECK (interval_months BETWEEN 1 AND 240),
    interval_days          INTEGER CHECK (interval_days BETWEEN 1 AND 3650),
    lead_km                INTEGER CHECK (lead_km >= 0),
    lead_days              INTEGER CHECK (lead_days >= 0),
    start_date             TEXT    NOT NULL,
    start_odometer_km      INTEGER NOT NULL CHECK (start_odometer_km >= 0),
    last_done_date         TEXT,
    last_done_odometer_km  INTEGER,
    next_due_km            INTEGER,
    next_due_date          TEXT,
    state                  TEXT    NOT NULL DEFAULT 'upcoming' CHECK (state IN ('upcoming', 'due_soon', 'overdue')),
    state_trigger          TEXT    CHECK (state_trigger IN ('km', 'time', 'hours')),
    state_changed_at       TEXT    NOT NULL,
    last_notified_state    TEXT    NOT NULL DEFAULT 'upcoming' CHECK (last_notified_state IN ('upcoming', 'due_soon', 'overdue')),
    paused_at              TEXT,
    created_by             INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at             TEXT    NOT NULL,
    updated_at             TEXT    NOT NULL,
    last_done_unknown      INTEGER NOT NULL DEFAULT 0 CHECK (last_done_unknown IN (0, 1)),
    hour_meter_id          INTEGER REFERENCES hour_meters (id) ON DELETE SET NULL,
    interval_hours         INTEGER CHECK (interval_hours BETWEEN 1 AND 100000),
    lead_hours             INTEGER CHECK (lead_hours >= 0),
    start_hours_tenths     INTEGER CHECK (start_hours_tenths >= 0),
    last_done_hours_tenths INTEGER,
    next_due_hours_tenths  INTEGER,
    CHECK (interval_km IS NOT NULL OR interval_months IS NOT NULL OR interval_days IS NOT NULL OR interval_hours IS NOT NULL),
    CHECK (NOT (interval_months IS NOT NULL AND interval_days IS NOT NULL)),
    CHECK (service_category_id IS NOT NULL OR length(title) > 0),
    CHECK (interval_hours IS NULL OR hour_meter_id IS NOT NULL),
    CHECK (lead_hours IS NULL OR interval_hours IS NOT NULL)
);

INSERT INTO reminders_new (id, vehicle_id, service_category_id, title, interval_km, interval_months, interval_days,
                           lead_km, lead_days, start_date, start_odometer_km, last_done_date, last_done_odometer_km,
                           next_due_km, next_due_date, state, state_trigger, state_changed_at, last_notified_state,
                           paused_at, created_by, created_at, updated_at, last_done_unknown)
SELECT id, vehicle_id, service_category_id, title, interval_km, interval_months, interval_days,
       lead_km, lead_days, start_date, start_odometer_km, last_done_date, last_done_odometer_km,
       next_due_km, next_due_date, state, state_trigger, state_changed_at, last_notified_state,
       paused_at, created_by, created_at, updated_at, last_done_unknown
FROM reminders;

DROP TABLE reminders;

ALTER TABLE reminders_new RENAME TO reminders;

CREATE INDEX reminders_vehicle_state ON reminders (vehicle_id, state);
CREATE INDEX reminders_category ON reminders (service_category_id);
CREATE INDEX reminders_hour_meter ON reminders (hour_meter_id);

COMMIT;

PRAGMA foreign_keys = ON;
