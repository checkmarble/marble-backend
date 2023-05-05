-- +goose Up
-- +goose StatementBegin
ALTER TABLE transactions DROP COLUMN bank_account_id;
ALTER TABLE transactions ADD COLUMN bank_account_id VARCHAR;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE transactions DROP COLUMN bank_account_id;
ALTER TABLE transactions ADD COLUMN bank_account_id uuid;

-- +goose StatementEnd