-- ============================================
-- File: 000055_auth_rbac_audit_hardening.sql
-- Hardens the RBAC + auth surface and turns audit_logs into a usable trail.
--
--  1. Adds a `roles.read` permission (browse the role/permission catalogue)
--     distinct from the existing `roles.manage` (mutate roles & grants).
--  2. Enriches audit_logs with denormalised actor/status/description columns
--     so the audit viewer can filter and render without extra joins.
--  3. Grants the new read permission to the administrative roles.
-- ============================================

-- +goose Up
-- +goose StatementBegin
INSERT INTO permissions (code, name, module, description) VALUES
  ('roles.read', 'Read roles', 'rbac', 'Can view roles, the permission catalogue and role membership'),
  ('users.delete', 'Delete users', 'users', 'Can soft-delete users')
ON CONFLICT (code) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
-- SUPER_ADMIN gets every permission (incl. these new ones); grant explicitly so
-- environments that skipped the original CROSS JOIN seed stay correct.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN ('roles.read', 'roles.manage', 'users.delete')
WHERE r.code = 'SUPER_ADMIN'
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
-- The delete route was referencing a permission that did not exist, so even
-- admins got 403. Grant users.delete to roles that already manage users.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'users.delete'
WHERE r.code = 'NATIONAL_ADMIN'
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
-- National / District admins may browse roles but not mutate them.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'roles.read'
WHERE r.code IN ('NATIONAL_ADMIN', 'DISTRICT_ADMIN')
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE audit_logs
  ADD COLUMN IF NOT EXISTS actor_username TEXT,
  ADD COLUMN IF NOT EXISTS actor_roles    TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS status         TEXT NOT NULL DEFAULT 'SUCCESS'
      CHECK (status IN ('SUCCESS', 'FAILURE', 'DENIED')),
  ADD COLUMN IF NOT EXISTS description    TEXT,
  ADD COLUMN IF NOT EXISTS request_id     TEXT,
  ADD COLUMN IF NOT EXISTS metadata       JSONB;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_status ON audit_logs(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_audit_logs_status;
DROP INDEX IF EXISTS idx_audit_logs_action;
ALTER TABLE audit_logs
  DROP COLUMN IF EXISTS metadata,
  DROP COLUMN IF EXISTS request_id,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS actor_roles,
  DROP COLUMN IF EXISTS actor_username;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('roles.read', 'users.delete'));
DELETE FROM permissions WHERE code IN ('roles.read', 'users.delete');
-- +goose StatementEnd
