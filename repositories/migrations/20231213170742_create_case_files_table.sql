-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS case_files (
      id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
      created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
      case_id UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
      bucket_name VARCHAR(255) NOT NULL,
      file_reference VARCHAR(255) NOT NULL,
      file_name VARCHAR(255) NOT NULL
);
CREATE UNIQUE INDEX case_files_unique_case_id_file_name ON case_files (case_id, bucket_name, file_reference);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE case_files;
-- +goose StatementEnd
