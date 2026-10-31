-- +goose Up
CREATE TABLE users (
    id              INTEGER PRIMARY KEY,
    username        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    display_name    TEXT    NOT NULL,
    role            TEXT    NOT NULL CHECK (role IN ('admin', 'member')),
    locale          TEXT    NOT NULL DEFAULT 'en' CHECK (locale IN ('en', 'de')),
    date_format     TEXT    NOT NULL DEFAULT 'YYYY-MM-DD' CHECK (date_format IN ('DD.MM.YYYY', 'YYYY-MM-DD')),
    number_format   TEXT    NOT NULL DEFAULT 'point' CHECK (number_format IN ('point', 'comma')),
    currency_code   TEXT    NOT NULL DEFAULT 'EUR',
    currency_symbol TEXT    CHECK (currency_symbol IS NULL OR length(currency_symbol) <= 5),
    disabled_at     TEXT    ,
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL
);

-- Credentials live apart from users so OIDC identities can be linked later
-- without touching any domain foreign key (Constitution IV).
CREATE TABLE user_identities (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider      TEXT    NOT NULL,
    subject       TEXT    NOT NULL,
    issuer        TEXT    ,
    password_hash TEXT    ,
    created_at    TEXT    NOT NULL,
    last_login_at TEXT    ,
    CHECK ((provider = 'local') = (password_hash IS NOT NULL)),
    UNIQUE (provider, subject)
);

CREATE UNIQUE INDEX user_identities_one_local ON user_identities (user_id) WHERE provider = 'local';

CREATE TABLE sessions (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   BLOB    NOT NULL UNIQUE,
    created_at   TEXT    NOT NULL,
    last_seen_at TEXT    NOT NULL,
    expires_at   TEXT    NOT NULL,
    user_agent   TEXT    NOT NULL DEFAULT '',
    ip           TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX sessions_user ON sessions (user_id);
