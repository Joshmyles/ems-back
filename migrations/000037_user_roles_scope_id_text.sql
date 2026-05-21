-- ============================================
-- File: 000037_user_roles_scope_id_text.sql
-- ============================================
-- Widens user_roles.scope_id from UUID to TEXT so it can reference either
-- UUID-keyed scopes (ref_districts, ref_subcounties, ref_facilities) or
-- TEXT-keyed scopes (facilities.facility_uid). Existing UUID values cast
-- losslessly to TEXT, and the RBAC repository already scans scope_id as
-- text — no consumer changes are required.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_roles ALTER COLUMN scope_id TYPE TEXT USING scope_id::text;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_roles ALTER COLUMN scope_id TYPE UUID USING scope_id::uuid;
-- +goose StatementEnd
