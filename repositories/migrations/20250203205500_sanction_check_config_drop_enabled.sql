-- +goose Up
-- +goose StatementBegin
ALTER TABLE sanction_check_configs
DROP COLUMN enabled;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE sanction_check_configs
ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose StatementEnd