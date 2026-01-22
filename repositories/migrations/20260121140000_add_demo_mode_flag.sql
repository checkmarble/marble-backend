-- +goose Up
ALTER TABLE organizations
ADD COLUMN environment text NOT NULL DEFAULT 'production' check (environment IN ('production', 'demo'));

-- +goose Down
ALTER TABLE organizations
DROP COLUMN environment;