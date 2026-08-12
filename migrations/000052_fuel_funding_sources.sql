-- ============================================
-- File: 000052_fuel_funding_sources.sql
-- Fuel funding sources: pots of money (e.g. donor/government allocations)
-- that fuel purchases can optionally draw from. A fuel log linked to a source
-- reduces that source's remaining balance (computed as amount - spent).
-- ============================================
-- +goose Up
CREATE TABLE fuel_funding_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_name TEXT NOT NULL,
    funding_date DATE NOT NULL DEFAULT CURRENT_DATE,
    amount NUMERIC(14,2) NOT NULL CHECK (amount >= 0),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE fuel_logs
    ADD COLUMN IF NOT EXISTS funding_source_id UUID REFERENCES fuel_funding_sources(id) ON DELETE SET NULL;

CREATE INDEX idx_fuel_logs_funding_source_id ON fuel_logs(funding_source_id);

-- +goose Down
ALTER TABLE fuel_logs DROP COLUMN IF EXISTS funding_source_id;
DROP TABLE IF EXISTS fuel_funding_sources;
