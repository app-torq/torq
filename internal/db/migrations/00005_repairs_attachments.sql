-- +goose Up
CREATE TABLE repair_logs (
    id                     INTEGER PRIMARY KEY,
    vehicle_id             INTEGER NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    title                  TEXT    NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    status                 TEXT    NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'in_progress', 'waiting_on_parts', 'completed')),
    symptom                TEXT    NOT NULL DEFAULT '',
    root_cause             TEXT    NOT NULL DEFAULT '',
    procedure_notes        TEXT    NOT NULL DEFAULT '' CHECK (length(procedure_notes) <= 50000),
    special_tools          TEXT    NOT NULL DEFAULT '',
    started_on             TEXT,
    completed_on           TEXT,
    completion_odometer_km INTEGER CHECK (completion_odometer_km >= 0),
    created_by             INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at             TEXT    NOT NULL,
    updated_at             TEXT    NOT NULL,
    CHECK (status <> 'completed' OR (completed_on IS NOT NULL AND completion_odometer_km IS NOT NULL)),
    CHECK (completed_on IS NULL OR started_on IS NULL OR completed_on >= started_on)
);

CREATE INDEX repair_logs_vehicle_status ON repair_logs (vehicle_id, status);
CREATE INDEX repair_logs_vehicle_completed ON repair_logs (vehicle_id, completed_on);

CREATE TABLE repair_fault_codes (
    id            INTEGER PRIMARY KEY,
    repair_log_id INTEGER NOT NULL REFERENCES repair_logs (id) ON DELETE CASCADE,
    code          TEXT    NOT NULL,
    UNIQUE (repair_log_id, code)
);

CREATE INDEX repair_fault_codes_code ON repair_fault_codes (code);

CREATE TABLE repair_torque_specs (
    id            INTEGER PRIMARY KEY,
    repair_log_id INTEGER NOT NULL REFERENCES repair_logs (id) ON DELETE CASCADE,
    label         TEXT    NOT NULL,
    value_nm      REAL    NOT NULL CHECK (value_nm > 0),
    note          TEXT    NOT NULL DEFAULT '',
    sort_order    INTEGER NOT NULL
);

CREATE INDEX repair_torque_specs_repair ON repair_torque_specs (repair_log_id, sort_order);

CREATE TABLE repair_parts (
    id                      INTEGER PRIMARY KEY,
    repair_log_id           INTEGER NOT NULL REFERENCES repair_logs (id) ON DELETE CASCADE,
    name                    TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    oem_part_number         TEXT    NOT NULL DEFAULT '',
    aftermarket_brand       TEXT    NOT NULL DEFAULT '',
    aftermarket_part_number TEXT    NOT NULL DEFAULT '',
    supplier_url            TEXT    NOT NULL DEFAULT '',
    quantity                INTEGER NOT NULL DEFAULT 1 CHECK (quantity >= 1),
    unit_cost_cents         INTEGER CHECK (unit_cost_cents >= 0),
    cost_type               TEXT    NOT NULL DEFAULT 'parts' CHECK (cost_type IN ('parts', 'fluids')),
    status                  TEXT    NOT NULL DEFAULT 'needed' CHECK (status IN ('needed', 'ordered', 'in_stock', 'installed')),
    sort_order              INTEGER NOT NULL
);

CREATE INDEX repair_parts_repair ON repair_parts (repair_log_id);
CREATE INDEX repair_parts_oem ON repair_parts (oem_part_number);
CREATE INDEX repair_parts_aftermarket ON repair_parts (aftermarket_part_number);

-- One table, one explicit nullable FK per owner (Constitution II: explicit references).
CREATE TABLE attachments (
    id             INTEGER PRIMARY KEY,
    storage_key    TEXT    NOT NULL UNIQUE,
    original_name  TEXT    NOT NULL CHECK (length(original_name) <= 255),
    mime_type      TEXT    NOT NULL CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp', 'image/heic', 'image/heif', 'application/pdf')),
    size_bytes     INTEGER NOT NULL,
    purpose        TEXT    NOT NULL DEFAULT 'file' CHECK (purpose IN ('file', 'avatar')),
    vehicle_id     INTEGER REFERENCES vehicles (id) ON DELETE CASCADE,
    service_log_id INTEGER REFERENCES service_logs (id) ON DELETE CASCADE,
    repair_log_id  INTEGER REFERENCES repair_logs (id) ON DELETE CASCADE,
    spec_entry_id  INTEGER REFERENCES vehicle_spec_entries (id) ON DELETE CASCADE,
    caption        TEXT    NOT NULL DEFAULT '',
    uploaded_by    INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at     TEXT    NOT NULL,
    CHECK ((vehicle_id IS NOT NULL) + (service_log_id IS NOT NULL) + (repair_log_id IS NOT NULL) + (spec_entry_id IS NOT NULL) = 1),
    CHECK ((purpose = 'avatar') = (vehicle_id IS NOT NULL))
);

CREATE UNIQUE INDEX attachments_one_avatar ON attachments (vehicle_id) WHERE purpose = 'avatar';
CREATE INDEX attachments_service_log ON attachments (service_log_id);
CREATE INDEX attachments_repair_log ON attachments (repair_log_id);
CREATE INDEX attachments_spec_entry ON attachments (spec_entry_id);

ALTER TABLE odometer_readings ADD COLUMN repair_log_id INTEGER REFERENCES repair_logs (id) ON DELETE CASCADE;
