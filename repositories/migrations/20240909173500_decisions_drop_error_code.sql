-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions
ALTER COLUMN error_code
DROP NOT NULL;

-- +goose StatementEnd
-- +goose Down
ALTER TABLE decisions
ALTER COLUMN error_code
SET NOT NULL;

-- +goose StatementBegin
-- +goose StatementEnd