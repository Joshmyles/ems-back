-- ============================================
-- File: 000041_mass_casualties.sql
-- Adds the Mass Casualties incident type and a casualty_count field on
-- incidents, used instead of individual patient details for such incidents.
-- ============================================
-- +goose Up
INSERT INTO ref_incident_types (code, name, description, requires_transport)
VALUES ('MASS_CASUALTIES', 'Mass Casualties', 'Incident involving multiple casualties', TRUE)
ON CONFLICT (code) DO NOTHING;

ALTER TABLE incidents ADD COLUMN IF NOT EXISTS casualty_count INT;

-- +goose Down
ALTER TABLE incidents DROP COLUMN IF EXISTS casualty_count;
DELETE FROM ref_incident_types WHERE code = 'MASS_CASUALTIES';
