-- ============================================
-- File: 000058_analytics_indexes.sql
-- Supporting indexes for the analytics & reports module. The incident
-- lifecycle is reconstructed from STATUS_CHANGE events, crew workload from the
-- driver/medic columns, and adoption from session/notification timestamps.
-- ============================================
-- +goose Up
CREATE INDEX IF NOT EXISTS idx_incident_updates_status_change
    ON incident_updates (incident_id, new_value, created_at)
    WHERE update_type = 'STATUS_CHANGE';

CREATE INDEX IF NOT EXISTS idx_dispatch_assignments_driver_user_id
    ON dispatch_assignments (driver_user_id);

CREATE INDEX IF NOT EXISTS idx_dispatch_assignments_lead_medic_user_id
    ON dispatch_assignments (lead_medic_user_id);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_created_at
    ON auth_sessions (created_at);

CREATE INDEX IF NOT EXISTS idx_notifications_created_at
    ON notifications (created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_notifications_created_at;
DROP INDEX IF EXISTS idx_auth_sessions_created_at;
DROP INDEX IF EXISTS idx_dispatch_assignments_lead_medic_user_id;
DROP INDEX IF EXISTS idx_dispatch_assignments_driver_user_id;
DROP INDEX IF EXISTS idx_incident_updates_status_change;
