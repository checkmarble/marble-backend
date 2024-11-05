-- +goose Up
-- +goose StatementBegin
ALTER TABLE organizations
ADD COLUMN IF NOT EXISTS use_marble_db_schema_as_default BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE organizations
DROP COLUMN use_marble_db_schema_as_default;

-- +goose StatementEnd