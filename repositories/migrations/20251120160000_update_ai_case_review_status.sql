-- +goose Up
-- +goose StatementBegin
ALTER TABLE ai_case_reviews DROP CONSTRAINT status_check;
ALTER TABLE ai_case_reviews ADD CONSTRAINT status_check CHECK (status IN ('pending', 'completed', 'failed', 'insufficient_funds'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE ai_case_reviews SET status = 'failed' WHERE status = 'insufficient_funds';
ALTER TABLE ai_case_reviews DROP CONSTRAINT status_check;
ALTER TABLE ai_case_reviews ADD CONSTRAINT status_check CHECK (status IN ('pending', 'completed', 'failed'));
-- +goose StatementEnd
