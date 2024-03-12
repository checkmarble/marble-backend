-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions
ADD COLUMN scenario_iteration_id UUID REFERENCES scenario_iterations (id);

ALTER TABLE decision_rules
ADD COLUMN rule_id UUID REFERENCES scenario_iteration_rules (id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE decisions
DROP COLUMN scenario_iteration_id;

ALTER TABLE decision_rules
DROP COLUMN rule_id;

-- +goose StatementEnd