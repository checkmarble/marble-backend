-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenarios
ADD COLUMN decision_to_case_name_template JSON;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenarios
DROP COLUMN decision_to_case_name_template;
-- +goose StatementEnd
