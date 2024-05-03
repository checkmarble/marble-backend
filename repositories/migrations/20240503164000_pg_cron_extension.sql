-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION pg_cron;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP EXTENSION pg_cron;

-- +goose StatementEnd