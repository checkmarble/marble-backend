-- +goose Up
ALTER TABLE cases ADD COLUMN review_level text;
ALTER TABLE cases ADD CONSTRAINT valid_review_level CHECK (review_level IN ('probable_false_positive', 'investigate', 'escalate'));

-- +goose Down
ALTER TABLE cases DROP CONSTRAINT valid_review_level;
ALTER TABLE cases DROP COLUMN review_level;
