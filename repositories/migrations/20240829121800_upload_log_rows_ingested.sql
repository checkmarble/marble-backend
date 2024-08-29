-- +goose Up
-- +goose StatementBegin
ALTER TABLE upload_logs
ADD COLUMN num_rows_ingested INTEGER NOT NULL DEFAULT 0;

UPDATE upload_logs
SET
    num_rows_ingested = lines_processed
WHERE
    status = 'success';

-- +goose StatementEnd
-- +goose Down
ALTER TABLE upload_logs
DROP COLUMN num_rows_ingested;

-- +goose StatementBegin
-- +goose StatementEnd