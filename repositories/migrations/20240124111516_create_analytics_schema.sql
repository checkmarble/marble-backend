-- +goose Up
-- +goose StatementBegin
-- create the analytics schema and an analytics user
CREATE SCHEMA IF NOT EXISTS analytics;

do $$
begin
   execute 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA analytics TO ' || current_user;
end
$$;

DO $$ BEGIN
      CREATE USER analytics;

EXCEPTION
      WHEN duplicate_object THEN RAISE NOTICE '%, skipping',
      SQLERRM USING ERRCODE = SQLSTATE;

END $$;

GRANT
SELECT
      ON ALL TABLES IN SCHEMA analytics TO analytics;

GRANT USAGE ON SCHEMA analytics TO analytics;

ALTER DEFAULT PRIVILEGES IN SCHEMA analytics
GRANT
SELECT
      ON TABLES TO analytics;

ALTER ROLE analytics
SET
      search_path TO analytics;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS analytics CASCADE;

-- +goose StatementEnd