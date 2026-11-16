-- +goose Up
-- Per-vehicle display units for displacement and power (#18). NULL means the
-- vehicle follows each user's preference; values stay stored in cm³ and W.
ALTER TABLE vehicles ADD COLUMN displacement_unit TEXT CHECK (displacement_unit IN ('cc', 'l', 'ci'));
ALTER TABLE vehicles ADD COLUMN power_unit TEXT CHECK (power_unit IN ('kw', 'ps', 'hp'));
