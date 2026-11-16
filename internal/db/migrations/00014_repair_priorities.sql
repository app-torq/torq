-- +goose Up
-- Project priorities (#15): an ordered list per garage that repair projects
-- can reference, plus how each user wants them shown. Built-ins are
-- translated through builtin_key until someone renames them (custom_name).
CREATE TABLE repair_priorities (
    id          INTEGER PRIMARY KEY,
    builtin_key TEXT    UNIQUE,
    custom_name TEXT    CHECK (custom_name IS NULL OR length(custom_name) BETWEEN 1 AND 40),
    color       TEXT    NOT NULL,
    sort_order  INTEGER NOT NULL,
    is_default  INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL,
    CHECK (builtin_key IS NOT NULL OR custom_name IS NOT NULL)
);

CREATE UNIQUE INDEX repair_priorities_custom_name ON repair_priorities (custom_name COLLATE NOCASE) WHERE custom_name IS NOT NULL;
CREATE UNIQUE INDEX repair_priorities_one_default ON repair_priorities (is_default) WHERE is_default = 1;

INSERT INTO repair_priorities (builtin_key, color, sort_order, is_default, created_at, updated_at) VALUES
    ('urgent', 'red',   1, 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    ('high',   'amber', 2, 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    ('normal', 'sky',   3, 1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    ('low',    'zinc',  4, 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

-- Existing projects keep no priority; deleting a priority unsets it.
ALTER TABLE repair_logs ADD COLUMN priority_id INTEGER REFERENCES repair_priorities (id) ON DELETE SET NULL;
CREATE INDEX repair_logs_priority ON repair_logs (priority_id);

ALTER TABLE users ADD COLUMN priority_display TEXT NOT NULL DEFAULT 'both' CHECK (priority_display IN ('both', 'color', 'name'));
