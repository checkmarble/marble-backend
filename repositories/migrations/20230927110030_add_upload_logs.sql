-- +goose Up
-- +goose StatementBegin
CREATE TABLE upload_logs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE NOT NULL,
  user_id UUID REFERENCES users NOT NULL,
  file_name VARCHAR NOT NULL,
  status VARCHAR NOT NULL,
  started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMP WITH TIME ZONE,
  lines_processed INTEGER NOT NULL DEFAULT 0
)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE upload_logs;
-- +goose StatementEnd
