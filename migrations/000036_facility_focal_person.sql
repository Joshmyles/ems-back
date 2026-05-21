-- ============================================
-- File: 000036_facility_focal_person.sql
-- ============================================
-- Adds a single focal_person_id to each facility — the designated user who
-- serves as the facility contact/coordinator. A facility may have many staff
-- but only one focal person. ON DELETE SET NULL so deleting the user does
-- not cascade to the facility row.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE facilities ADD COLUMN focal_person_id UUID REFERENCES users(id) ON DELETE SET NULL;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_facilities_focal_person_id ON facilities(focal_person_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_facilities_focal_person_id;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE facilities DROP COLUMN focal_person_id;
-- +goose StatementEnd
