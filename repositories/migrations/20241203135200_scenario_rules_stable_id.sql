-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_iteration_rules
ADD COLUMN stable_rule_id uuid;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_iteration_rules
DROP COLUMN stable_rule_id;

-- +goose StatementEnd