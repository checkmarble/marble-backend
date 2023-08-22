-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_iterations ADD COLUMN trigger_condition_ast_expression json;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_iterations DROP COLUMN trigger_condition_ast_expression;
-- +goose StatementEnd
