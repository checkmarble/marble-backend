-- +goose Up
-- +goose StatementBegin
DROP INDEX decision_pivot_id_idx;

DROP INDEX decisions_scenario_iteration_id_idx;

-- +goose StatementEnd
-- +goose Down
CREATE INDEX CONCURRENTLY IF NOT EXISTS decision_pivot_id_idx ON decisions (pivot_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS decisions_scenario_iteration_id_idx ON decisions (scenario_iteration_id);