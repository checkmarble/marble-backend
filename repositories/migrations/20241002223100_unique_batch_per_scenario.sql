-- +goose Up
-- +goose StatementBegin
CREATE INDEX unique_scheduled_per_scenario_idx ON scheduled_executions (scenario_id)
WHERE
    (status IN ('pending', 'processing'));

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX unique_scheduled_per_scenario_idx;

-- +goose StatementEnd