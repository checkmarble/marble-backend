-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenarios
DROP COLUMN decision_to_case_inbox_id,
DROP COLUMN decision_to_case_outcomes,
DROP COLUMN decision_to_case_workflow_type,
DROP COLUMN decision_to_case_name_template;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenarios
ADD COLUMN decision_to_case_inbox_id UUID REFERENCES inboxes (id) ON DELETE SET NULL ON UPDATE CASCADE;

ALTER TABLE scenarios
ADD COLUMN decision_to_case_outcomes varchar(50) [];

ALTER TABLE scenarios
ADD COLUMN decision_to_case_workflow_type VARCHAR(255) NOT NULL DEFAULT 'DISABLED';

ALTER TABLE scenarios
ADD COLUMN decision_to_case_name_template JSON;

-- +goose StatementEnd