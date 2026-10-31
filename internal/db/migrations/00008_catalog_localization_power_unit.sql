-- +goose Up
-- Localized catalog names, chassis-code generations and admin-triggered
-- online catalog updates (Constitution 2.0.0, Principle VII), plus the
-- display-only kW/PS preference (Hard Non-Goals, "Metric only").

-- German display names; the seeder fills them with the English name when a
-- name is the same in both languages. *_de_key mirror *_key for searching.
ALTER TABLE catalog_vehicles ADD COLUMN model_de TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_vehicles ADD COLUMN model_de_key TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_vehicles ADD COLUMN generation_de TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_vehicles ADD COLUMN generation_de_key TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_kba ADD COLUMN model_de TEXT NOT NULL DEFAULT '';

-- source: where the imported catalog came from. bundled_sha256: the last
-- bundled seed startup has seen, so an online update survives restarts until
-- a release ships a newer bundled catalog.
ALTER TABLE catalog_meta ADD COLUMN source TEXT NOT NULL DEFAULT 'bundled' CHECK (source IN ('bundled', 'online'));
ALTER TABLE catalog_meta ADD COLUMN bundled_sha256 TEXT NOT NULL DEFAULT '';

-- Force a re-import of the bundled seed so the new columns get filled.
UPDATE catalog_meta SET seed_sha256 = '', bundled_sha256 = '' WHERE id = 1;

ALTER TABLE users ADD COLUMN power_unit TEXT NOT NULL DEFAULT 'kw' CHECK (power_unit IN ('kw', 'ps'));

