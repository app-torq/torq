-- +goose Up
-- Second factors (#45). They are credentials: kept apart from the garage
-- data and removed with their user (Constitution IV).

-- An authenticator app. The secret is encrypted with the key in
-- <data>/secret.key; last_step is the last accepted 30-second interval, so a
-- code works only once.
CREATE TABLE user_totp (
    user_id      INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    secret_enc   BLOB    NOT NULL,
    confirmed_at TEXT,
    last_step    INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT    NOT NULL
);

-- Single-use recovery codes, stored as SHA-256 of the normalized code.
CREATE TABLE user_recovery_codes (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  BLOB    NOT NULL UNIQUE,
    used_at    TEXT,
    created_at TEXT    NOT NULL
);

CREATE INDEX user_recovery_codes_user ON user_recovery_codes (user_id);

-- The random WebAuthn user handle, so passkeys never carry the user id.
CREATE TABLE user_webauthn (
    user_id INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    handle  BLOB    NOT NULL UNIQUE
);

-- Passkeys: only public keys and counters, as the WebAuthn library's JSON.
CREATE TABLE user_passkeys (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    credential_id BLOB    NOT NULL UNIQUE,
    credential    TEXT    NOT NULL,
    name          TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    user_verified INTEGER NOT NULL DEFAULT 0 CHECK (user_verified IN (0, 1)),
    created_at    TEXT    NOT NULL,
    last_used_at  TEXT
);

CREATE INDEX user_passkeys_user ON user_passkeys (user_id);

ALTER TABLE users ADD COLUMN mfa_required INTEGER NOT NULL DEFAULT 0 CHECK (mfa_required IN (0, 1));
ALTER TABLE roles ADD COLUMN require_mfa INTEGER NOT NULL DEFAULT 0 CHECK (require_mfa IN (0, 1));
