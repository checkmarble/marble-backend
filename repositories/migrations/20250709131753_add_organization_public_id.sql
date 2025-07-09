-- +goose Up
-- +goose StatementBegin
ALTER TABLE organizations ADD COLUMN public_id UUID NOT NULL UNIQUE DEFAULT (uuid_generate_v4());
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Don't need to drop the column, it's not used anymore and it's not a big deal to keep it. We avoid recreating the column with different public ID

-- +goose StatementEnd
