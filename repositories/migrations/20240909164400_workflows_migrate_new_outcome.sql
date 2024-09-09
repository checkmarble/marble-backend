-- +goose Up
-- +goose StatementBegin
UPDATE scenarios
SET
    decision_to_case_outcomes = array_append(decision_to_case_outcomes, 'block_and_review')
WHERE
    'review' = ANY (decision_to_case_outcomes);

-- +goose StatementEnd
-- +goose Down
UPDATE scenarios
SET
    decision_to_case_outcomes = array_remove(decision_to_case_outcomes, 'block_and_review')
WHERE
    'block_and_review' = ANY (decision_to_case_outcomes);

-- +goose StatementBegin
-- +goose StatementEnd