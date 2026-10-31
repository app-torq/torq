-- +goose Up
CREATE TABLE webhooks (
    id                     INTEGER PRIMARY KEY,
    name                   TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    url                    TEXT    NOT NULL CHECK (length(url) <= 2048),
    preset                 TEXT    NOT NULL CHECK (preset IN ('discord', 'ntfy', 'gotify', 'home_assistant', 'generic_json')),
    enabled                INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    on_due_soon            INTEGER NOT NULL DEFAULT 0 CHECK (on_due_soon IN (0, 1)),
    on_overdue             INTEGER NOT NULL DEFAULT 0 CHECK (on_overdue IN (0, 1)),
    on_weekly_digest       INTEGER NOT NULL DEFAULT 0 CHECK (on_weekly_digest IN (0, 1)),
    on_monthly_digest      INTEGER NOT NULL DEFAULT 0 CHECK (on_monthly_digest IN (0, 1)),
    owner_user_id          INTEGER REFERENCES users (id) ON DELETE SET NULL,
    last_weekly_digest_on  TEXT,
    last_monthly_digest_on TEXT,
    created_at             TEXT    NOT NULL,
    updated_at             TEXT    NOT NULL
);

CREATE TABLE webhook_headers (
    id         INTEGER PRIMARY KEY,
    webhook_id INTEGER NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    value      TEXT    NOT NULL
);

CREATE UNIQUE INDEX webhook_headers_name ON webhook_headers (webhook_id, name COLLATE NOCASE);

-- Transactional outbox and delivery log (research R10).
CREATE TABLE webhook_deliveries (
    id                   INTEGER PRIMARY KEY,
    webhook_id           INTEGER NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event                TEXT    NOT NULL CHECK (event IN ('reminder_due_soon', 'reminder_overdue', 'digest_weekly', 'digest_monthly', 'test')),
    reminder_id          INTEGER REFERENCES reminders (id) ON DELETE SET NULL,
    request_body         BLOB    NOT NULL,
    request_headers_json TEXT    NOT NULL,
    status               TEXT    NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    attempt_count        INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count <= 4),
    next_attempt_at      TEXT,
    last_status_code     INTEGER,
    last_error           TEXT    NOT NULL DEFAULT '',
    response_excerpt     TEXT    NOT NULL DEFAULT '',
    duration_ms          INTEGER,
    created_at           TEXT    NOT NULL,
    completed_at         TEXT
);

CREATE INDEX webhook_deliveries_due ON webhook_deliveries (status, next_attempt_at);
CREATE INDEX webhook_deliveries_log ON webhook_deliveries (webhook_id, created_at DESC);
