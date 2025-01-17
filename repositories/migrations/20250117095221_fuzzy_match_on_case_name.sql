-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION pg_trgm;
ALTER DATABASE marble SET pg_trgm.similarity_threshold = 0.1;
CREATE INDEX trgm_cases_on_name ON cases USING GIN (name gin_trgm_ops);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX trgm_cases_on_name;
DROP EXTENSION pg_trgm;
-- +goose StatementEnd
