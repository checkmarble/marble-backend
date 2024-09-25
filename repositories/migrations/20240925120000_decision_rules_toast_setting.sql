-- +goose Up
-- +goose StatementBegin
ALTER TABLE decision_rules
SET
    (TOAST_TUPLE_TARGET = 128);

ALTER TABLE decision_rules
ALTER COLUMN name
DROP NOT NULL;

ALTER TABLE decision_rules
ALTER COLUMN description
DROP NOT NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE decision_rules
SET
    (TOAST_TUPLE_TARGET = 2048);

ALTER TABLE decision_rules
ALTER COLUMN name
SET NOT NULL;

ALTER TABLE decision_rules
ALTER COLUMN description
SET NOT NULL;

-- +goose StatementEnd