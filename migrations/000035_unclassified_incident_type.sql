-- ============================================
-- File: 000035_unclassified_incident_type.sql
-- ============================================
-- Seeds a sentinel UNCLASSIFIED incident type so public/citizen-submitted
-- reports (e.g. from the mobile app) can satisfy the required
-- incident_type_id field. Dispatch reclassifies the incident after triage.
-- The UUID is fixed so clients can send it as a default without first
-- having to look it up.

-- +goose Up
-- +goose StatementBegin
INSERT INTO ref_incident_types (id, code, name, description, requires_transport)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'UNCLASSIFIED',
    'Unclassified',
    'Default type for public/citizen reports pending dispatch classification',
    TRUE
)
ON CONFLICT (code) DO UPDATE
SET id = EXCLUDED.id,
    name = EXCLUDED.name,
    description = EXCLUDED.description;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM ref_incident_types WHERE code = 'UNCLASSIFIED';
-- +goose StatementEnd
