-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_iteration_rules ADD COLUMN formula_ast_expression json;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_iteration_rules DROP COLUMN formula_ast_expression;
-- +goose StatementEnd
