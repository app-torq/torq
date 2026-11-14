-- +goose NO TRANSACTION
-- +goose Up
-- Display units and US formats (Constitution 3.0.0, "Metric storage"):
-- values stay metric; users choose how distances, volumes, torque, pressure,
-- displacement and power are shown and entered. New users start with the
-- instance defaults.
--
-- users and service_log_items get looser CHECK constraints, so they are
-- rebuilt as in 00012 (foreign keys off, outside the transaction).
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE users_new (
    id                INTEGER PRIMARY KEY,
    username          TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    display_name      TEXT    NOT NULL,
    role              TEXT    NOT NULL CHECK (role IN ('admin', 'member')),
    locale            TEXT    NOT NULL DEFAULT 'en' CHECK (locale IN ('en', 'de')),
    date_format       TEXT    NOT NULL DEFAULT 'YYYY-MM-DD' CHECK (date_format IN ('DD.MM.YYYY', 'YYYY-MM-DD', 'MM/DD/YYYY')),
    number_format     TEXT    NOT NULL DEFAULT 'point' CHECK (number_format IN ('point', 'comma')),
    currency_code     TEXT    NOT NULL DEFAULT 'EUR',
    currency_symbol   TEXT    CHECK (currency_symbol IS NULL OR length(currency_symbol) <= 5),
    disabled_at       TEXT    ,
    created_at        TEXT    NOT NULL,
    updated_at        TEXT    NOT NULL,
    power_unit        TEXT    NOT NULL DEFAULT 'kw' CHECK (power_unit IN ('kw', 'ps', 'hp')),
    time_format       TEXT    NOT NULL DEFAULT '24h' CHECK (time_format IN ('24h', '12h')),
    distance_unit     TEXT    NOT NULL DEFAULT 'km' CHECK (distance_unit IN ('km', 'mi')),
    volume_unit       TEXT    NOT NULL DEFAULT 'l' CHECK (volume_unit IN ('l', 'qt')),
    torque_unit       TEXT    NOT NULL DEFAULT 'nm' CHECK (torque_unit IN ('nm', 'lbft')),
    pressure_unit     TEXT    NOT NULL DEFAULT 'bar' CHECK (pressure_unit IN ('bar', 'psi', 'kpa')),
    displacement_unit TEXT    NOT NULL DEFAULT 'cc' CHECK (displacement_unit IN ('cc', 'l', 'ci'))
);

INSERT INTO users_new (id, username, display_name, role, locale, date_format, number_format, currency_code,
                       currency_symbol, disabled_at, created_at, updated_at, power_unit)
SELECT id, username, display_name, role, locale, date_format, number_format, currency_code,
       currency_symbol, disabled_at, created_at, updated_at, power_unit
FROM users;

DROP TABLE users;

ALTER TABLE users_new RENAME TO users;

-- quantity_unit is the unit an amount was entered in, so it can be shown the
-- same way; quantity_ml stays the stored value.
CREATE TABLE service_log_items_new (
    id                      INTEGER PRIMARY KEY,
    service_log_id          INTEGER NOT NULL REFERENCES service_logs (id) ON DELETE CASCADE,
    category_id             INTEGER NOT NULL REFERENCES service_categories (id) ON DELETE RESTRICT,
    brand                   TEXT    NOT NULL DEFAULT '',
    specification           TEXT    NOT NULL DEFAULT '',
    quantity_ml             INTEGER CHECK (quantity_ml > 0),
    quantity_unit           TEXT    CHECK (quantity_unit IN ('L', 'mL', 'qt', 'gal', 'fl_oz')),
    oem_part_number         TEXT    NOT NULL DEFAULT '',
    aftermarket_part_number TEXT    NOT NULL DEFAULT '',
    cost_cents              INTEGER CHECK (cost_cents >= 0),
    cost_type               TEXT    NOT NULL CHECK (cost_type IN ('parts', 'fluids')),
    sort_order              INTEGER NOT NULL,
    CHECK ((quantity_ml IS NULL) = (quantity_unit IS NULL)),
    UNIQUE (service_log_id, category_id)
);

INSERT INTO service_log_items_new SELECT * FROM service_log_items;

DROP TABLE service_log_items;

ALTER TABLE service_log_items_new RENAME TO service_log_items;

CREATE INDEX service_log_items_category ON service_log_items (category_id, service_log_id);

ALTER TABLE instance_settings ADD COLUMN default_date_format TEXT NOT NULL DEFAULT 'YYYY-MM-DD' CHECK (default_date_format IN ('DD.MM.YYYY', 'YYYY-MM-DD', 'MM/DD/YYYY'));
ALTER TABLE instance_settings ADD COLUMN default_number_format TEXT NOT NULL DEFAULT 'point' CHECK (default_number_format IN ('point', 'comma'));
ALTER TABLE instance_settings ADD COLUMN default_time_format TEXT NOT NULL DEFAULT '24h' CHECK (default_time_format IN ('24h', '12h'));
ALTER TABLE instance_settings ADD COLUMN default_currency_code TEXT NOT NULL DEFAULT 'EUR';
ALTER TABLE instance_settings ADD COLUMN default_power_unit TEXT NOT NULL DEFAULT 'kw' CHECK (default_power_unit IN ('kw', 'ps', 'hp'));
ALTER TABLE instance_settings ADD COLUMN default_distance_unit TEXT NOT NULL DEFAULT 'km' CHECK (default_distance_unit IN ('km', 'mi'));
ALTER TABLE instance_settings ADD COLUMN default_volume_unit TEXT NOT NULL DEFAULT 'l' CHECK (default_volume_unit IN ('l', 'qt'));
ALTER TABLE instance_settings ADD COLUMN default_torque_unit TEXT NOT NULL DEFAULT 'nm' CHECK (default_torque_unit IN ('nm', 'lbft'));
ALTER TABLE instance_settings ADD COLUMN default_pressure_unit TEXT NOT NULL DEFAULT 'bar' CHECK (default_pressure_unit IN ('bar', 'psi', 'kpa'));
ALTER TABLE instance_settings ADD COLUMN default_displacement_unit TEXT NOT NULL DEFAULT 'cc' CHECK (default_displacement_unit IN ('cc', 'l', 'ci'));

COMMIT;

PRAGMA foreign_keys = ON;
