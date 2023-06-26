-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_iterations ALTER COLUMN batch_trigger_sql set default '';
ALTER TABLE scenario_iterations ALTER COLUMN schedule set default '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_iterations ALTER COLUMN batch_trigger_sql DROP DEFAULT;
ALTER TABLE scenario_iterations ALTER COLUMN schedule DROP DEFAULT;
-- +goose StatementEnd
