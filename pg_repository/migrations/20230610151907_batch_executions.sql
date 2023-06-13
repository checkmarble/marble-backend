-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenarios ADD COLUMN scenario_type VARCHAR;
ALTER TABLE scenario_iterations ADD COLUMN batch_trigger_sql VARCHAR;
ALTER TABLE scenario_iterations ADD COLUMN schedule VARCHAR;


CREATE TABLE scheduled_executions (
    id uuid DEFAULT uuid_generate_v4(),
    org_id uuid NOT NULL,
    scenario_id uuid NOT NULL,
    scenario_iteration_id uuid NOT NULL,
    status VARCHAR NOT NULL,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP,
    PRIMARY KEY(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenarios DROP COLUMN scenario_type;
ALTER TABLE scenario_iterations DROP COLUMN batch_trigger_sql;
ALTER TABLE scenario_iterations DROP COLUMN schedule;
DROP TABLE scheduled_executions;
-- +goose StatementEnd