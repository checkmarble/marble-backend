-- +goose Up
-- +goose StatementBegin
ALTER TABLE decision_rules
ADD COLUMN rule_evaluation jsonb;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE decision_rules
DROP COLUMN rule_evaluation;
-- +goose StatementEnd
