-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenarios
ADD COLUMN decision_to_case_inbox_id UUID REFERENCES inboxes (id) ON DELETE SET NULL ON UPDATE CASCADE;

ALTER TABLE scenarios
ADD COLUMN decision_to_case_outcomes varchar(50) [];

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenarios
DROP COLUMN decision_to_case_inbox_id;

ALTER TABLE scenarios
DROP COLUMN decision_to_case_outcomes;

-- +goose StatementEnd