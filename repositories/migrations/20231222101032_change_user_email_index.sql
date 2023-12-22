-- +goose Up
-- +goose StatementBegin
DROP INDEX users_email_idx;
CREATE UNIQUE INDEX users_email_org_idx ON users (email, organization_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX users_email_org_idx;
CREATE UNIQUE INDEX users_email_idx ON users (email);
-- +goose StatementEnd
