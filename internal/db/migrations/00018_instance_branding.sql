-- +goose Up
-- Instance name, logo and contact details (#32). The name and logo appear in
-- the app header, on the sign-in page and on the printed history; contact
-- details only on the printed history. Empty strings mean "not set".
ALTER TABLE instance_settings ADD COLUMN brand_name TEXT NOT NULL DEFAULT '' CHECK (length(brand_name) <= 60);
ALTER TABLE instance_settings ADD COLUMN brand_address TEXT NOT NULL DEFAULT '' CHECK (length(brand_address) <= 300);
ALTER TABLE instance_settings ADD COLUMN brand_phone TEXT NOT NULL DEFAULT '' CHECK (length(brand_phone) <= 40);
ALTER TABLE instance_settings ADD COLUMN brand_email TEXT NOT NULL DEFAULT '' CHECK (length(brand_email) <= 120);
ALTER TABLE instance_settings ADD COLUMN brand_website TEXT NOT NULL DEFAULT '' CHECK (length(brand_website) <= 200);

-- The logo is small (at most 512 KB), so it lives in the database rather than
-- in the attachments directory, in its own table so that reading the settings
-- does not load it.
CREATE TABLE instance_logo (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    mime_type  TEXT    NOT NULL CHECK (mime_type IN ('image/png', 'image/jpeg', 'image/svg+xml')),
    data       BLOB    NOT NULL,
    updated_at TEXT    NOT NULL
);
