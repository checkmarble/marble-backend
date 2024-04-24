-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions
ADD COLUMN pivot_id uuid REFERENCES data_model_pivots (id) ON DELETE SET NULL;

ALTER TABLE decisions
ADD COLUMN pivot_value text;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE decisions
DROP COLUMN pivot_id;

ALTER TABLE decisions
DROP COLUMN pivot_value;

-- +goose StatementEnd