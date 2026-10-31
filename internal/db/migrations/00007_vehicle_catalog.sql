-- +goose Up
-- Vehicle Auto-Lookup & Cascading Catalog (specs/002-vehicle-catalog-lookup, data-model.md).
-- Vehicles keep plain values; nothing references the catalog tables (Constitution VII).
ALTER TABLE vehicles ADD COLUMN hsn TEXT CHECK (hsn IS NULL OR (length(hsn) = 4 AND hsn NOT GLOB '*[^0-9]*'));
ALTER TABLE vehicles ADD COLUMN tsn TEXT CHECK (tsn IS NULL OR (length(tsn) = 3 AND tsn NOT GLOB '*[^0-9A-Z]*'));
ALTER TABLE vehicles ADD COLUMN displacement_cc INTEGER CHECK (displacement_cc IS NULL OR displacement_cc BETWEEN 1 AND 20000);
ALTER TABLE vehicles ADD COLUMN power_kw INTEGER CHECK (power_kw IS NULL OR power_kw BETWEEN 1 AND 2000);
ALTER TABLE vehicles ADD COLUMN fuel_type TEXT CHECK (fuel_type IS NULL OR fuel_type IN ('diesel', 'petrol', 'hybrid', 'electric', 'lpg', 'cng', 'other'));

CREATE TABLE catalog_meta (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    seed_sha256 TEXT    NOT NULL,
    imported_at TEXT    NOT NULL,
    header_json TEXT    NOT NULL
);

INSERT INTO catalog_meta (id, seed_sha256, imported_at, header_json)
VALUES (1, '', '1970-01-01T00:00:00Z', '{"datasets":[],"counts":{}}');

CREATE TABLE catalog_kba (
    id              INTEGER PRIMARY KEY,
    hsn             TEXT    NOT NULL,
    tsn             TEXT    NOT NULL,
    make            TEXT    NOT NULL,
    model           TEXT    NOT NULL,
    trade_name      TEXT    NOT NULL,
    generation      TEXT    NOT NULL DEFAULT '',
    engine_code     TEXT    NOT NULL DEFAULT '',
    displacement_cc INTEGER,
    power_kw        INTEGER,
    fuel_type       TEXT    CHECK (fuel_type IS NULL OR fuel_type IN ('diesel', 'petrol', 'hybrid', 'electric', 'lpg', 'cng', 'other')),
    body_code       TEXT    NOT NULL DEFAULT '',
    year_from       INTEGER,
    source          TEXT    NOT NULL CHECK (source IN ('sv42', 'fz6')),
    match           TEXT    NOT NULL CHECK (match IN ('unique', 'none'))
);

CREATE INDEX catalog_kba_key ON catalog_kba (hsn, tsn);

CREATE TABLE catalog_vehicles (
    id              INTEGER PRIMARY KEY,
    make            TEXT    NOT NULL,
    make_key        TEXT    NOT NULL,
    model           TEXT    NOT NULL,
    model_key       TEXT    NOT NULL,
    generation      TEXT    NOT NULL,
    generation_key  TEXT    NOT NULL,
    year_from       INTEGER,
    year_to         INTEGER,
    body            TEXT    NOT NULL DEFAULT '',
    engine          TEXT    NOT NULL,
    engine_key      TEXT    NOT NULL,
    engine_code     TEXT    NOT NULL DEFAULT '',
    displacement_cc INTEGER,
    power_kw        INTEGER,
    fuel_type       TEXT    CHECK (fuel_type IS NULL OR fuel_type IN ('diesel', 'petrol', 'hybrid', 'electric', 'lpg', 'cng', 'other'))
);

CREATE INDEX catalog_vehicles_make ON catalog_vehicles (make_key, make);
CREATE INDEX catalog_vehicles_model ON catalog_vehicles (make_key, model_key, model);
CREATE INDEX catalog_vehicles_generation ON catalog_vehicles (make_key, model_key, generation_key, generation, year_from, year_to);
CREATE INDEX catalog_vehicles_engine ON catalog_vehicles (make_key, model_key, generation_key, engine_key);
