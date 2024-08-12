-- +goose Up
-- +goose StatementBegin
ALTER TABLE rule_snoozes
ADD COLUMN created_from_rule_id UUID REFERENCES scenario_iteration_rules (id) ON DELETE SET NULL;

-- +goose StatementEnd
-- +goose Down
ALTER TABLE rule_snoozes
DROP COLUMN created_from_rule_id;

-- +goose StatementBegin
-- +goose StatementEnd