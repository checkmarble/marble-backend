-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions ADD COLUMN scheduled_execution_id uuid;
CREATE INDEX decisions_scheduled_execution_id_idx ON decisions(scheduled_execution_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX decisions_scheduled_execution_id_idx;
ALTER TABLE decisions DROP COLUMN scheduled_execution_id;
-- +goose StatementEnd
