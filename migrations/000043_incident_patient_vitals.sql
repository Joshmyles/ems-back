-- ============================================
-- File: 000039_incident_patient_vitals.sql
-- ============================================
-- Adds current patient vitals captured at incident intake. Stored as TEXT so
-- free-form clinical entries are preserved exactly as recorded (e.g. blood
-- pressure as "120/80", temperature as "37.2"). All columns are nullable —
-- existing rows keep NULL and the API coalesces to '' on read.
--   * respiratory_rate - breaths per minute
--   * spo2             - oxygen saturation (%)
--   * pulse            - heart rate (bpm)
--   * bp               - blood pressure (systolic/diastolic)
--   * temperature      - body temperature (°C)

-- +goose Up
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN respiratory_rate TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN spo2 TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN pulse TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN bp TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN temperature TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN respiratory_rate;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN spo2;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN pulse;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN bp;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN temperature;
-- +goose StatementEnd
