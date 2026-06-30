-- ============================================
-- File: 000049_users_staff_no_seq.sql
-- ============================================
-- Adds a sequence used to auto-generate user staff numbers. staff_no is a
-- UNIQUE column, so it can no longer be left blank: an empty string collides
-- with the next blank one (only NULLs are allowed to repeat). New users now
-- receive a server-generated value like 'EMS-000042' when none is supplied,
-- removing the need for any client to invent (and risk duplicating) one.

-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS users_staff_no_seq START WITH 1000 INCREMENT BY 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SEQUENCE IF EXISTS users_staff_no_seq;
-- +goose StatementEnd
