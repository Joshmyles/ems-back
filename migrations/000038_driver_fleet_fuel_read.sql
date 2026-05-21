-- ============================================
-- File: 000038_driver_fleet_fuel_read.sql
-- ============================================
-- Grants the fleet.read and fuel.read permissions to the DRIVER role so
-- drivers can view ambulances and fuel logs from the EMS app/front. The
-- original RBAC seed only gave drivers dispatch.read, dispatch.update_status,
-- and trips.read, which caused 403s on the fleet and fuel endpoints.

-- +goose Up
-- +goose StatementBegin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN ('fleet.read', 'fuel.read')
WHERE r.code = 'DRIVER'
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE code = 'DRIVER')
  AND permission_id IN (SELECT id FROM permissions WHERE code IN ('fleet.read', 'fuel.read'));
-- +goose StatementEnd
