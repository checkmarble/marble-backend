-- +goose Up
-- +goose StatementBegin
CREATE INDEX decisions_scenario_iteration_id_idx ON decisions (scenario_iteration_id);

CREATE INDEX decision_rules_rule_id_idx ON decision_rules (rule_id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX decisions_scenario_iteration_id_idx;

DROP INDEX decision_rules_rule_id_idx;

-- +goose StatementEnd