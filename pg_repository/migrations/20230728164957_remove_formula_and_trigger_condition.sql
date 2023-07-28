-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_iterations DROP COLUMN trigger_condition;
ALTER TABLE scenario_iteration_rules DROP COLUMN formula;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_iteration_rules ADD COLUMN formula json NOT NULL;
ALTER TABLE scenario_iterations ADD COLUMN trigger_condition json;
-- +goose StatementEnd
