-- +goose Up
-- +goose StatementBegin
ALTER TABLE scheduled_executions ADD COLUMN manual boolean;
CREATE INDEX scheduled_executions_scenario_id_idx ON scheduled_executions(scenario_id);
CREATE INDEX scheduled_executions_organization_id_idx ON scheduled_executions(organization_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scheduled_executions DROP COLUMN manual;
DROP INDEX scheduled_executions_scenario_id_idx
DROP INDEX scenario_executions_organization_id_idx;
-- +goose StatementEnd
