-- +goose Up
-- What a role sees (#49): every role that exists keeps seeing everything, so
-- nothing changes until an administrator takes something away. The
-- Administrator role's permissions aren't stored; it sees everything anyway.
INSERT OR IGNORE INTO role_permissions (role_id, permission)
SELECT r.id, p.value
FROM roles r
CROSS JOIN json_each('["costs.view", "service_logs.view", "repairs.view", "reminders.view", "reports.view",
                       "attachments.view", "notes.view", "vehicle_ids.view"]') AS p
WHERE r.builtin_key IS NULL OR r.builtin_key != 'admin';
