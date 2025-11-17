-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_workflow_rules
ADD COLUMN
type TEXT NOT NULL DEFAULT 'decision';

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_workflow_rules
DROP COLUMN
type;

-- +goose StatementEnd