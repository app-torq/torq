-- +goose Up
-- The label a client sends at sign-in (e.g. an app's device name), shown in
-- the Devices list. Browser sessions leave it empty.
ALTER TABLE sessions ADD COLUMN device_name TEXT NOT NULL DEFAULT '';
