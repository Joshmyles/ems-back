-- ============================================
-- File: 000057_fuel_funding_topups.sql
-- Turns a fuel funding source into a living budget: beyond its initial amount,
-- money can be added over time via top-ups. Each top-up is an auditable row.
--
--   total funded = amount + SUM(top-ups)
--   remaining    = total funded - spent (sum of linked fuel-log costs)
-- ============================================

-- +goose Up
CREATE TABLE fuel_funding_topups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    funding_source_id UUID NOT NULL REFERENCES fuel_funding_sources(id) ON DELETE CASCADE,
    amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    topup_date DATE NOT NULL DEFAULT CURRENT_DATE,
    notes TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fuel_funding_topups_source
    ON fuel_funding_topups(funding_source_id, topup_date DESC, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS fuel_funding_topups;
