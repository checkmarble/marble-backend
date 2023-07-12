-- +goose Up
-- +goose StatementBegin
ALTER TABLE custom_lists ADD UNIQUE (organization_id, name)-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
