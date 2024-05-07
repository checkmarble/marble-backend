-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenarios
ADD COLUMN decision_to_case_workflow_type VARCHAR(255);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenarios
DROP COLUMN decision_to_case_workflow_type;

-- +goose StatementEnd