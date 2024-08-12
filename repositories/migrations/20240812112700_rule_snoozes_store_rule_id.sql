-- +goose Up
-- +goose StatementBegin
TRUNCATE rule_snoozes;

ALTER TABLE rule_snoozes
ADD COLUMN created_from_rule_id UUID NOT NULL REFERENCES scenario_iteration_rules (id) ON DELETE CASCADE;

-- +goose StatementEnd
-- +goose Down
ALTER TABLE rule_snoozes
DROP COLUMN created_from_rule_id;

-- +goose StatementBegin
-- +goose StatementEnd