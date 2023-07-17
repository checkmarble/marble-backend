-- +goose Up
-- +goose StatementBegin
ALTER TABLE custom_lists ADD CONSTRAINT custom_lists_organization_id_name_key UNIQUE (organization_id, name)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
ALTER TABLE custom_lists DROP CONSTRAINT custom_lists_organization_id_name_key;
