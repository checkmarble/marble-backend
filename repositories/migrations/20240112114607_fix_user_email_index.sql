-- +goose Up
-- +goose StatementBegin
DROP INDEX users_email_idx;

CREATE UNIQUE INDEX users_email_idx ON users (email)
WHERE deleted_at IS NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX users_email_idx;

CREATE UNIQUE INDEX users_email_idx ON users (email)
WHERE deleted_at IS NOT NULL;

-- +goose StatementEnd