-- +goose Up
-- +goose StatementBegin
ALTER TABLE licenses
ADD COLUMN rule_snoozes BOOL NOT NULL DEFAULT FALSE;

-- +goose StatementEnd
-- +goose Down
ALTER TABLE licenses
DROP COLUMN rule_snoozes;

-- +goose StatementBegin
-- +goose StatementEnd