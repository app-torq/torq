-- +goose Up
CREATE TABLE instance_settings (
    id                    INTEGER PRIMARY KEY CHECK (id = 1),
    weekly_digest_weekday INTEGER NOT NULL DEFAULT 1 CHECK (weekly_digest_weekday BETWEEN 1 AND 7),
    weekly_digest_hour    INTEGER NOT NULL DEFAULT 8 CHECK (weekly_digest_hour BETWEEN 0 AND 23),
    monthly_digest_day    INTEGER NOT NULL DEFAULT 1 CHECK (monthly_digest_day BETWEEN 1 AND 28),
    monthly_digest_hour   INTEGER NOT NULL DEFAULT 8 CHECK (monthly_digest_hour BETWEEN 0 AND 23)
);

INSERT INTO instance_settings (id) VALUES (1);
