-- +goose Up
-- +goose StatementBegin
ALTER TABLE ai_case_reviews
ADD COLUMN file_temp_reference TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ai_case_reviews
DROP COLUMN file_temp_reference;

-- +goose StatementEnd
