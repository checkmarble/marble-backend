-- +goose Up
-- +goose StatementBegin
ALTER TABLE licenses
ADD COLUMN webhooks BOOL NOT NULL DEFAULT FALSE;

-- +goose StatementEnd
-- +goose Down
ALTER TABLE licenses
DROP COLUMN webhooks;

-- +goose StatementBegin
-- +goose StatementEnd