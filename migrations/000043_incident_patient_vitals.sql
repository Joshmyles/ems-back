-- ============================================
-- File: 000043_incident_patient_vitals.sql
-- ============================================
-- NOTE: renumbered from 000039 to resolve a duplicate-version collision with
-- 000045_facility_focal_person_incident_update.sql (both were authored as 39 on
-- separate branches). Columns use IF NOT EXISTS / IF EXISTS so re-application is
-- a safe no-op if an earlier deploy already added them under the old version.
--
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
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS respiratory_rate TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS spo2 TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS pulse TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS bp TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS temperature TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN IF EXISTS respiratory_rate;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN IF EXISTS spo2;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN IF EXISTS pulse;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN IF EXISTS bp;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE incidents DROP COLUMN IF EXISTS temperature;
-- +goose StatementEnd
