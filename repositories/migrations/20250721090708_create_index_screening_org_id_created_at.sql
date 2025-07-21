-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY sanction_checks_created_at_idx ON sanction_checks (created_at DESC);

-- +goose Down
DROP INDEX CONCURRENTLY sanction_checks_created_at_idx;
