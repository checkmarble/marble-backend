-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
ALTER TABLE upload_logs ADD COLUMN table_name VARCHAR NOT NULL DEFAULT '';
CREATE INDEX idx_table_name_org_id ON upload_logs (table_name, org_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE upload_logs DROP COLUMN table_name;
DROP INDEX idx_table_name_org_id;
-- +goose StatementEnd
