-- ============================================
-- File: 000040_driver_dispatcher_trips_manage.sql
-- ============================================
-- Allows drivers and dispatchers to record and update trip logs.

-- +goose Up
-- +goose StatementBegin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'trips.manage'
WHERE r.code IN ('DRIVER', 'DISPATCHER')
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE code IN ('DRIVER', 'DISPATCHER'))
  AND permission_id IN (SELECT id FROM permissions WHERE code = 'trips.manage');
-- +goose StatementEnd
