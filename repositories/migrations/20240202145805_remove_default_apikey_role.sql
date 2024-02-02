-- +goose Up
-- +goose StatementBegin
ALTER TABLE apikeys ALTER COLUMN role DROP DEFAULT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE apikeys ALTER COLUMN role SET DEFAULT 5;
-- +goose StatementEnd
