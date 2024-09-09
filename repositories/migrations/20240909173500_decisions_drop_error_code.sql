-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions
DROP COLUMN error_code;

-- +goose StatementEnd
-- +goose Down
ALTER TABLE decisions
ADD COLUMN error_code INT NOT NULL DEFAULT 0;

-- +goose StatementBegin
-- +goose StatementEnd