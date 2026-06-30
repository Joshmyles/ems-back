-- ============================================
-- File: 000050_incident_number_seq.sql
-- ============================================
-- Adds a sequence for generating incident numbers. The previous scheme derived
-- the running number from COUNT(*) of incidents, which is neither atomic (two
-- concurrent creates read the same count) nor stable across deletions (deleting
-- a row lowers the count, so the next create reuses an existing number). Both
-- produce a duplicate-key violation on incidents_incident_number_key. A sequence
-- hands out monotonic values atomically, immune to deletes and races.
--
-- The sequence is seeded just above the highest suffix already present so newly
-- generated numbers can never collide with existing rows.

-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS incidents_incident_number_seq;
-- +goose StatementEnd
-- +goose StatementBegin
SELECT setval(
    'incidents_incident_number_seq',
    COALESCE(
        (
            SELECT MAX(split_part(incident_number, '-', 3)::bigint)
            FROM incidents
            WHERE incident_number ~ '^INC-[0-9]{8}-[0-9]+$'
        ),
        0
    ) + 1,
    false
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SEQUENCE IF EXISTS incidents_incident_number_seq;
-- +goose StatementEnd
