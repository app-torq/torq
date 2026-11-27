-- +goose Up
-- Users share their own vehicles (#50): every assignment records how it came
-- about. Existing ones were made by an administrator (or when a user added the
-- vehicle, which can't be told apart any more; both may be shared the same).
-- granted_by is set only for shares; deleting the sharer ends their shares.
ALTER TABLE vehicle_access ADD COLUMN origin TEXT NOT NULL DEFAULT 'admin'
    CHECK (origin IN ('admin', 'created', 'shared'));
ALTER TABLE vehicle_access ADD COLUMN granted_by INTEGER REFERENCES users (id) ON DELETE CASCADE;
CREATE INDEX vehicle_access_granted_by ON vehicle_access (granted_by);
