-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN first_name VARCHAR;
ALTER TABLE users  ADD COLUMN last_name VARCHAR;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN first_name;
ALTER TABLE users DROP COLUMN last_name;
-- +goose StatementEnd
