-- ============================================
-- File: 000054_blood_requisition_decision.sql
-- Adds APPROVED / DECLINED statuses so an admin can record an explicit
-- decision on a blood requisition before (or instead of) broadcasting.
-- ============================================
-- +goose Up
ALTER TABLE blood_requisitions DROP CONSTRAINT IF EXISTS blood_requisitions_status_check;
ALTER TABLE blood_requisitions ADD CONSTRAINT blood_requisitions_status_check
    CHECK (status IN ('OPEN', 'APPROVED', 'DECLINED', 'BROADCASTING', 'MATCHED', 'PICKUP_ASSIGNED', 'COLLECTED', 'DELIVERED', 'CANCELLED', 'EXPIRED'));

-- +goose Down
UPDATE blood_requisitions SET status = 'OPEN' WHERE status IN ('APPROVED', 'DECLINED');
ALTER TABLE blood_requisitions DROP CONSTRAINT IF EXISTS blood_requisitions_status_check;
ALTER TABLE blood_requisitions ADD CONSTRAINT blood_requisitions_status_check
    CHECK (status IN ('OPEN', 'BROADCASTING', 'MATCHED', 'PICKUP_ASSIGNED', 'COLLECTED', 'DELIVERED', 'CANCELLED', 'EXPIRED'));
