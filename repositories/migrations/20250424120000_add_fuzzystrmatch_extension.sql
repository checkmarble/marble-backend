-- +goose Up
CREATE EXTENSION if not exists fuzzystrmatch SCHEMA public;
ALTER EXTENSION pg_trgm SET SCHEMA public;

-- +goose Down
DROP EXTENSION if exists fuzzystrmatch;
ALTER EXTENSION pg_trgm SET SCHEMA marble;