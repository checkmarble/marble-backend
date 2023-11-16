-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX custom_list_unique_name_idx ON custom_lists (organization_id, name) WHERE deleted_at IS NULL;
ALTER TABLE custom_lists DROP CONSTRAINT custom_lists_organization_id_name_key;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX custom_list_unique_name_idx;
ALTER TABLE custom_lists ADD CONSTRAINT custom_lists_organization_id_name_key UNIQUE (organization_id, name)
-- +goose StatementEnd
