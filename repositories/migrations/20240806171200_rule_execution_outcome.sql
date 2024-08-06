-- +goose Up
-- +goose StatementBegin
ALTER TABLE decision_rules
ADD COLUMN outcome VARCHAR(10) NOT NULL DEFAULT '';

-- +goose StatementEnd
-- +goose Down
ALTER TABLE decision_rules
DROP COLUMN outcome;

-- +goose StatementBegin
-- +goose StatementEnd