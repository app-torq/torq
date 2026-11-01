-- +goose Up
-- Per-field provenance of the structured spec sheet: whether a value was
-- filled automatically and from which web source (specs auto-fill). JSON
-- object keyed by spec field: {"engine_oil_spec": {"is_auto": true,
-- "source_url": "https://…", "source_name": "example.com"}}. Fields without an
-- entry were entered by hand.
ALTER TABLE vehicle_specs ADD COLUMN specs_meta TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(specs_meta));
