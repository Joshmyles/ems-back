-- ============================================
-- File: 000039_facility_focal_person_incident_update.sql
-- ============================================
-- Allows facility focal persons to update incident details, status, and triage
-- fields through endpoints guarded by incidents.triage.

-- +goose Up
-- +goose StatementBegin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'incidents.triage'
WHERE r.code = 'FACILITY_FOCAL_PERSON'
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE code = 'FACILITY_FOCAL_PERSON')
  AND permission_id IN (SELECT id FROM permissions WHERE code = 'incidents.triage');
-- +goose StatementEnd
