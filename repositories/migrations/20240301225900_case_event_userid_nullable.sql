-- +goose Up
-- +goose StatementBegin
ALTER TABLE case_events
ALTER COLUMN user_id
DROP NOT NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE case_events
ALTER COLUMN user_id
SET NOT NULL;

-- +goose StatementEnd