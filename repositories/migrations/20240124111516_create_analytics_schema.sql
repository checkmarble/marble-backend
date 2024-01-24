-- +goose Up
-- +goose StatementBegin
-- create and make default the marble schema
CREATE SCHEMA IF NOT EXISTS analytics;

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA analytics TO postgres;

DO $$
BEGIN
CREATE USER analytics WITH PASSWORD 'default';
EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
END
$$;

GRANT SELECT ON ALL TABLES IN SCHEMA analytics TO analytics
GRANT USAGE ON SCHEMA analytics TO analytics
ALTER DEFAULT PRIVILEGES IN SCHEMA analytics GRANT SELECT ON TABLES TO analytics;

ALTER ROLE analytics
SET search_path TO analytics;


-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS analytics CASCADE;

-- +goose StatementEnd