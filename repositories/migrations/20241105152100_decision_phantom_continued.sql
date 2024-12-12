-- +goose Up
-- +goose StatementBegin
ALTER TABLE phantom_decisions
ADD COLUMN scenario_version INT NOT NULL;

ALTER TABLE phantom_decisions
ADD COLUMN test_run_id uuid NOT NULL REFERENCES scenario_test_run (id) ON DELETE CASCADE;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE phantom_decisions
DROP COLUMN scenario_version;

ALTER TABLE phantom_decisions
DROP COLUMN test_run_id;

-- +goose StatementEnd