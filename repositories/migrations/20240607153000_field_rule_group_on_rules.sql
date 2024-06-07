-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_iteration_rules
ADD COLUMN rule_group VARCHAR(255) NOT NULL DEFAULT '';

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_iteration_rules
DROP COLUMN rule_group;

-- +goose StatementEnd