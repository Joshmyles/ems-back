-- ============================================
-- File: 000040_incident_delete_permission.sql
-- ============================================
-- Adds the incidents.delete permission used to guard hard-deletion of an
-- incident. Deletion is destructive (it cascades to dispatch, triage, trips and
-- incident updates) so it is restricted to administrator roles only:
-- SUPER_ADMIN, NATIONAL_ADMIN and DISTRICT_ADMIN. SUPER_ADMIN must be granted
-- explicitly because the original CROSS JOIN seed only covered permissions that
-- existed at that time.

-- +goose Up
-- +goose StatementBegin
INSERT INTO permissions (code, name, module, description) VALUES
('incidents.delete', 'Delete incidents', 'incidents', 'Can delete incidents')
ON CONFLICT (code) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'incidents.delete'
WHERE r.code IN ('SUPER_ADMIN', 'NATIONAL_ADMIN', 'DISTRICT_ADMIN')
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM role_permissions rp
USING permissions p
WHERE rp.permission_id = p.id
  AND p.code = 'incidents.delete';
-- +goose StatementEnd
-- +goose StatementBegin
DELETE FROM permissions WHERE code = 'incidents.delete';
-- +goose StatementEnd
