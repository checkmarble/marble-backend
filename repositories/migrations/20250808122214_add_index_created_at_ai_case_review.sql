-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX concurrently idx_ai_case_reviews_created_at ON ai_case_reviews (created_at DESC);

-- +goose Down
DROP INDEX idx_ai_case_reviews_created_at;
