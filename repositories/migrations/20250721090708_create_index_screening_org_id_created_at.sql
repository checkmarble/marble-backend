-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY sanction_checks_decision_created_at_idx ON sanction_checks (decision_id, created_at DESC);
-- +goose Down
DROP INDEX CONCURRENTLY sanction_checks_decision_created_at_idx;
