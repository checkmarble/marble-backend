-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenarios ADD COLUMN scenario_type VARCHAR;
ALTER TABLE scenario_iterations ADD COLUMN batch_trigger_sql VARCHAR;
ALTER TABLE scenario_iterations ADD COLUMN schedule VARCHAR;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenarios DROP COLUMN scenario_type;
ALTER TABLE scenario_iterations DROP COLUMN batch_trigger_sql;
ALTER TABLE scenario_iterations DROP COLUMN schedule;
-- +goose StatementEnd
