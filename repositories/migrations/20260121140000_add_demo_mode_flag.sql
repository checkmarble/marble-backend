-- +goose Up
ALTER TABLE organizations ADD COLUMN demo_mode BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE organizations DROP COLUMN demo_mode;
