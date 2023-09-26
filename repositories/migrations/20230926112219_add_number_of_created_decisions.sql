-- +goose Up
-- +goose StatementBegin
ALTER TABLE scheduled_executions ADD COLUMN number_of_created_decisions INTEGER NOT NULL DEFAULT -1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scheduled_executions DROP COLUMN number_of_created_decisions;
-- +goose StatementEnd
