-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions
ADD COLUMN review_status VARCHAR(10);

-- +goose StatementEnd
-- +goose Down
ALTER TABLE decisions
DROP COLUMN review_status;

-- +goose StatementBegin
-- +goose StatementEnd