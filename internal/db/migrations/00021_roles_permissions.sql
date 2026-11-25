-- +goose Up
-- Roles and permissions: every user has one role, a named set of permissions
-- plus whether it sees all vehicles or only assigned ones. Built-ins are
-- translated through builtin_key until someone renames them (custom_name).
-- The Administrator role's permissions aren't stored: it may do everything.
CREATE TABLE roles (
    id           INTEGER PRIMARY KEY,
    builtin_key  TEXT    UNIQUE CHECK (builtin_key IS NULL OR builtin_key IN ('admin', 'member', 'viewer')),
    custom_name  TEXT    CHECK (custom_name IS NULL OR length(custom_name) BETWEEN 1 AND 40),
    description  TEXT    NOT NULL DEFAULT '' CHECK (length(description) <= 200),
    all_vehicles INTEGER NOT NULL DEFAULT 1 CHECK (all_vehicles IN (0, 1)),
    created_at   TEXT    NOT NULL,
    updated_at   TEXT    NOT NULL,
    CHECK (builtin_key IS NOT NULL OR custom_name IS NOT NULL)
);

CREATE UNIQUE INDEX roles_custom_name ON roles (custom_name COLLATE NOCASE) WHERE custom_name IS NOT NULL;

-- Permission keys are validated in Go, so the list can grow.
CREATE TABLE role_permissions (
    role_id    INTEGER NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission TEXT    NOT NULL,
    PRIMARY KEY (role_id, permission)
) WITHOUT ROWID;

-- Vehicles a user sees when their role doesn't see all vehicles; edit lets
-- the role's per-vehicle permissions apply, view is read only.
CREATE TABLE vehicle_access (
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vehicle_id INTEGER NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    level      TEXT    NOT NULL CHECK (level IN ('view', 'edit')),
    created_at TEXT    NOT NULL,
    PRIMARY KEY (user_id, vehicle_id)
) WITHOUT ROWID;

CREATE INDEX vehicle_access_vehicle ON vehicle_access (vehicle_id);

INSERT INTO roles (builtin_key, all_vehicles, created_at, updated_at) VALUES
    ('admin',  1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    ('member', 1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    ('viewer', 1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

-- Member keeps exactly what members could do: everything except the catalog
-- update, which was administrator-only.
INSERT INTO role_permissions (role_id, permission)
SELECT (SELECT id FROM roles WHERE builtin_key = 'member'), p.value
FROM json_each('["vehicles.edit", "vehicles.delete", "specs.edit", "service_logs.write", "service_logs.delete",
                 "reminders.write", "repairs.write", "repairs.delete", "shares.manage", "history.export",
                 "vehicles.create", "categories.manage", "repair_lists.manage"]') AS p;

-- users.role (admin/member) stays for older clients and the last-admin rule;
-- Go keeps it in sync with role_id: admin for the Administrator role, member
-- for every other. role_id can't be NOT NULL here (SQLite can't add such a
-- column with a foreign key); Go always sets it.
ALTER TABLE users ADD COLUMN role_id INTEGER REFERENCES roles (id);
CREATE INDEX users_role ON users (role_id);
UPDATE users SET role_id = (SELECT id FROM roles WHERE roles.builtin_key = users.role);
