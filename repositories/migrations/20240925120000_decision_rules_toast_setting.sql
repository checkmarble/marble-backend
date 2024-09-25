-- +goose Up
-- +goose StatementBegin
ALTER TABLE decision_rules
SET
    (TOAST_TUPLE_TARGET = 128);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE decision_rules
SET
    (TOAST_TUPLE_TARGET = 2048);

-- +goose StatementEnd