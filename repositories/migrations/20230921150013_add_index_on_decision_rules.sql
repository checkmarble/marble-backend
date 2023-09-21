-- +goose Up
-- +goose StatementBegin
CREATE INDEX decision_rules_decisionId_idx ON decision_rules(decision_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX decision_rules_decisionId_idx;
-- +goose StatementEnd