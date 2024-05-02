-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
ADD COLUMN partner_id UUID REFERENCES partners (id) ON DELETE CASCADE;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
DROP COLUMN partner_id;

-- +goose StatementEnd