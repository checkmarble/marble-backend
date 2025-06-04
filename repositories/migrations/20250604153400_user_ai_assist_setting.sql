-- +goose Up
-- +goose StatementBegin

ALTER TABLE users ADD COLUMN ai_assist_enabled BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE
users DROP COLUMN ai_assist_enabled;

-- +goose StatementEnd