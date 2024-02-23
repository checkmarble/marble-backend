-- +goose Up
-- +goose StatementBegin
--- those views are created outside if this migration file, but we need to drop it here becauses it uses the database_name column
-- (it wil be recreated as the migrations script is run)
DROP VIEW IF EXISTS analytics.organizations;

ALTER TABLE organizations
DROP COLUMN database_name;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE organizations
ADD COLUMN database_name VARCHAR NOT NULL;

-- +goose StatementEnd