-- +goose Up
-- +goose StatementBegin
ALTER TABLE ai_case_reviews
ADD COLUMN comment text;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ai_case_reviews
DROP COLUMN comment;
-- +goose StatementEnd
