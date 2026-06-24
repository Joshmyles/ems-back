-- ============================================
-- File: 000047_incident_delete_cascade_fix.sql
-- ============================================
-- Deleting an incident cascades into dispatch_assignments (and trips, triage,
-- recommendations, feedback). However several foreign keys that point at
-- incidents / dispatch_assignments were created with the default NO ACTION
-- delete rule, so they block the cascade with errors such as:
--
--   update or delete on table "dispatch_assignments" violates foreign key
--   constraint "fk_user_availability_current_dispatch_assignment" on table
--   "user_availability"
--
-- This migration repoints those FKs to ON DELETE SET NULL so an incident
-- delete cascades all the way through:
--   * user_availability.current_incident_id / current_dispatch_assignment_id
--     are live "currently working on" pointers — clearing them (not deleting
--     the responder's availability row) is the correct behaviour.
--   * the communications logs (inbound_sms, outbound_sms, ussd_sessions,
--     call_logs) keep their audit record but unlink from the deleted incident,
--     mirroring the existing blood_requisitions.incident_id ON DELETE SET NULL.

-- +goose Up
ALTER TABLE user_availability DROP CONSTRAINT IF EXISTS fk_user_availability_current_dispatch_assignment;
ALTER TABLE user_availability
    ADD CONSTRAINT fk_user_availability_current_dispatch_assignment
    FOREIGN KEY (current_dispatch_assignment_id) REFERENCES dispatch_assignments(id) ON DELETE SET NULL;

ALTER TABLE user_availability DROP CONSTRAINT IF EXISTS fk_user_availability_current_incident;
ALTER TABLE user_availability
    ADD CONSTRAINT fk_user_availability_current_incident
    FOREIGN KEY (current_incident_id) REFERENCES incidents(id) ON DELETE SET NULL;

ALTER TABLE inbound_sms DROP CONSTRAINT IF EXISTS inbound_sms_linked_incident_id_fkey;
ALTER TABLE inbound_sms
    ADD CONSTRAINT inbound_sms_linked_incident_id_fkey
    FOREIGN KEY (linked_incident_id) REFERENCES incidents(id) ON DELETE SET NULL;

ALTER TABLE outbound_sms DROP CONSTRAINT IF EXISTS outbound_sms_linked_incident_id_fkey;
ALTER TABLE outbound_sms
    ADD CONSTRAINT outbound_sms_linked_incident_id_fkey
    FOREIGN KEY (linked_incident_id) REFERENCES incidents(id) ON DELETE SET NULL;

ALTER TABLE ussd_sessions DROP CONSTRAINT IF EXISTS ussd_sessions_linked_incident_id_fkey;
ALTER TABLE ussd_sessions
    ADD CONSTRAINT ussd_sessions_linked_incident_id_fkey
    FOREIGN KEY (linked_incident_id) REFERENCES incidents(id) ON DELETE SET NULL;

ALTER TABLE call_logs DROP CONSTRAINT IF EXISTS call_logs_linked_incident_id_fkey;
ALTER TABLE call_logs
    ADD CONSTRAINT call_logs_linked_incident_id_fkey
    FOREIGN KEY (linked_incident_id) REFERENCES incidents(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE user_availability DROP CONSTRAINT IF EXISTS fk_user_availability_current_dispatch_assignment;
ALTER TABLE user_availability
    ADD CONSTRAINT fk_user_availability_current_dispatch_assignment
    FOREIGN KEY (current_dispatch_assignment_id) REFERENCES dispatch_assignments(id);

ALTER TABLE user_availability DROP CONSTRAINT IF EXISTS fk_user_availability_current_incident;
ALTER TABLE user_availability
    ADD CONSTRAINT fk_user_availability_current_incident
    FOREIGN KEY (current_incident_id) REFERENCES incidents(id);

ALTER TABLE inbound_sms DROP CONSTRAINT IF EXISTS inbound_sms_linked_incident_id_fkey;
ALTER TABLE inbound_sms
    ADD CONSTRAINT inbound_sms_linked_incident_id_fkey
    FOREIGN KEY (linked_incident_id) REFERENCES incidents(id);

ALTER TABLE outbound_sms DROP CONSTRAINT IF EXISTS outbound_sms_linked_incident_id_fkey;
ALTER TABLE outbound_sms
    ADD CONSTRAINT outbound_sms_linked_incident_id_fkey
    FOREIGN KEY (linked_incident_id) REFERENCES incidents(id);

ALTER TABLE ussd_sessions DROP CONSTRAINT IF EXISTS ussd_sessions_linked_incident_id_fkey;
ALTER TABLE ussd_sessions
    ADD CONSTRAINT ussd_sessions_linked_incident_id_fkey
    FOREIGN KEY (linked_incident_id) REFERENCES incidents(id);

ALTER TABLE call_logs DROP CONSTRAINT IF EXISTS call_logs_linked_incident_id_fkey;
ALTER TABLE call_logs
    ADD CONSTRAINT call_logs_linked_incident_id_fkey
    FOREIGN KEY (linked_incident_id) REFERENCES incidents(id);
