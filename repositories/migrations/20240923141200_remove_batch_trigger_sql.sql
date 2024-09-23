-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_iterations
DROP COLUMN batch_trigger_sql;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_iterations
ADD COLUMN batch_trigger_sql VARCHAR DEFAULT '';

-- +goose StatementEnd