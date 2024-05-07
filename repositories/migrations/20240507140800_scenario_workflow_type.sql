-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenarios
ADD COLUMN decision_to_case_workflow_type VARCHAR(255) NOT NULL DEFAULT 'DISABLED';

UPDATE scenarios
SET
      decision_to_case_workflow_type = 'CREATE_CASE'
WHERE
      decision_to_case_inbox_id IS NOT NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenarios
DROP COLUMN decision_to_case_workflow_type;

-- +goose StatementEnd