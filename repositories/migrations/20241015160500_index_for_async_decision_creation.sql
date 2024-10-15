-- +goose Up
-- +goose StatementBegin
CREATE INDEX decisions_to_create_query_pending_idx ON decisions_to_create (scheduled_execution_id)
WHERE
    status IN ('pending', 'failed');

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX decisions_to_create_query_pending_idx;

-- +goose StatementEnd