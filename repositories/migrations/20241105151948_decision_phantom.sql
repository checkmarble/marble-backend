-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions
ADD COLUMN test_run_id uuid NULL,
ADD CONSTRAINT fk_decision_test_run_id FOREIGN KEY(test_run_id) REFERENCES test_run(id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE decisions DROP COLUMN test_run_id;
-- +goose StatementEnd
