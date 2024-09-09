-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_iterations
ADD COLUMN score_block_and_review_threshold INT2;

UPDATE scenario_iterations
SET
    score_block_and_review_threshold = score_reject_threshold
WHERE
    score_block_and_review_threshold IS NULL;

ALTER TABLE scenario_iterations
ALTER COLUMN score_block_and_review_threshold
SET NOT NULL;

ALTER TYPE decision_outcome
RENAME VALUE 'null' TO 'block_and_review';

-- +goose StatementEnd
-- +goose Down
ALTER TABLE scenario_iterations
DROP COLUMN score_block_and_review_threshold;

ALTER TYPE decision_outcome
RENAME VALUE 'block_and_review' TO 'null';

-- +goose StatementBegin
-- +goose StatementEnd