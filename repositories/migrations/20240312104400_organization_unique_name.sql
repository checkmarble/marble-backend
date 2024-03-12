-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX organization_name_unique_idx ON organizations (name)
WHERE
      deleted_at IS NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX organization_name_unique_idx;

-- +goose StatementEnd