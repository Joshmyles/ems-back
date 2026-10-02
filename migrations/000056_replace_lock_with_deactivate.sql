-- ============================================
-- File: 000056_replace_lock_with_deactivate.sql
-- Replaces the account-locking concept with simple activate/deactivate.
--
--   * Any locked account becomes deactivated (is_active = false).
--   * status is reduced to ACTIVE / INACTIVE and kept in step with is_active.
--   * the is_locked column is removed; failed_login_attempts is kept for
--     display only (logins no longer auto-lock).
-- ============================================

-- +goose Up
-- +goose StatementBegin
-- Locked accounts (or legacy SUSPENDED/LOCKED statuses) become deactivated.
UPDATE users SET is_active = false WHERE is_locked = true;
UPDATE users SET status = CASE WHEN is_active THEN 'ACTIVE' ELSE 'INACTIVE' END;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_check;
ALTER TABLE users ADD CONSTRAINT users_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS is_locked;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_locked BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_check;
ALTER TABLE users ADD CONSTRAINT users_status_check
    CHECK (status IN ('ACTIVE', 'INACTIVE', 'SUSPENDED', 'LOCKED'));
-- +goose StatementEnd
