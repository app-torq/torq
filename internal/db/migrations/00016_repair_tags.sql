-- +goose Up
-- Project tags (#16): one shared tag list per garage, no colors; a project
-- can have any number of them. Names are unique regardless of case, and the
-- search separators (comma, double quote) are kept out by the app.
CREATE TABLE repair_tags (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE COLLATE NOCASE CHECK (length(name) BETWEEN 1 AND 40),
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL
);

CREATE TABLE repair_log_tags (
    repair_log_id INTEGER NOT NULL REFERENCES repair_logs (id) ON DELETE CASCADE,
    tag_id        INTEGER NOT NULL REFERENCES repair_tags (id) ON DELETE CASCADE,
    PRIMARY KEY (repair_log_id, tag_id)
);

CREATE INDEX repair_log_tags_tag ON repair_log_tags (tag_id, repair_log_id);

-- How many tags a project card shows before "+ n more"; 0 shows all.
ALTER TABLE users ADD COLUMN card_tag_limit INTEGER NOT NULL DEFAULT 3 CHECK (card_tag_limit BETWEEN 0 AND 20);
