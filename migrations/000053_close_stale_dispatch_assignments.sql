-- ============================================
-- File: 000053_close_stale_dispatch_assignments.sql
-- Data repair: dispatch assignments were never closed when their incident
-- reached a terminal status (or was reverted to AWAITING_ASSIGNMENT), leaving
-- "open" rows that block re-dispatch via uq_open_dispatch_per_incident.
-- Completed incidents get COMPLETED assignments; everything else stale is
-- CANCELLED. Going forward the application keeps the two in sync.
-- ============================================
-- +goose Up
UPDATE dispatch_assignments da
SET status = 'COMPLETED', updated_at = now()
FROM incidents i
WHERE i.id = da.incident_id
  AND i.status = 'COMPLETED'
  AND da.status IN ('PROPOSED','ASSIGNED','ACCEPTED','DEPARTED','ARRIVED_SCENE','PATIENT_LOADED');

UPDATE dispatch_assignments da
SET status = 'CANCELLED',
    cancellation_reason = COALESCE(da.cancellation_reason, 'Closed by data repair: incident was ' || i.status),
    updated_at = now()
FROM incidents i
WHERE i.id = da.incident_id
  AND i.status IN ('CANCELLED','REJECTED','AWAITING_ASSIGNMENT')
  AND da.status IN ('PROPOSED','ASSIGNED','ACCEPTED','DEPARTED','ARRIVED_SCENE','PATIENT_LOADED');

-- +goose Down
-- Data repair; no rollback.
SELECT 1;
