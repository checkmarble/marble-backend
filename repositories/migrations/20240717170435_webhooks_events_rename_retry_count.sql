-- +goose Up
-- +goose StatementBegin
ALTER TABLE webhook_events
RENAME COLUMN send_attempt_count TO retry_count;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE webhook_events
RENAME COLUMN retry_count TO send_attempt_count;

-- +goose StatementEnd