-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenarios
ADD COLUMN decision_to_case_inbox_id UUID;

ALTER TABLE scenarios
ADD COLUMN decision_to_case_outcomes decision_outcome[];

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenarios
DROP COLUMN decision_to_case_inbox_id;

ALTER TABLE scenarios
DROP COLUMN decision_to_case_outcomes;

-- +goose StatementEnd