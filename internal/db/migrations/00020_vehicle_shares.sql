-- +goose Up
-- Public read-only links per vehicle (#38). The token stays readable so the
-- garage can reprint labels; lookups only ever use its SHA-256, so the time a
-- lookup takes reveals nothing about which tokens exist. Turning a link off
-- deletes the row; turning it on again creates a new token.
CREATE TABLE vehicle_shares (
    vehicle_id            INTEGER PRIMARY KEY REFERENCES vehicles (id) ON DELETE CASCADE,
    token                 TEXT    NOT NULL,
    token_sha256          BLOB    NOT NULL UNIQUE,
    include_costs         INTEGER NOT NULL DEFAULT 0 CHECK (include_costs IN (0, 1)),
    include_license_plate INTEGER NOT NULL DEFAULT 0 CHECK (include_license_plate IN (0, 1)),
    include_vin           INTEGER NOT NULL DEFAULT 0 CHECK (include_vin IN (0, 1)),
    created_by            INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at            TEXT    NOT NULL,
    last_opened_at        TEXT
);

-- Administrators can turn every public link off at once.
ALTER TABLE instance_settings ADD COLUMN public_links_enabled INTEGER NOT NULL DEFAULT 1 CHECK (public_links_enabled IN (0, 1));
