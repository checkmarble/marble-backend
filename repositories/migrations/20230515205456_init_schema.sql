-- +goose Up
-- +goose StatementBegin
-- create and make default the marble schema
CREATE SCHEMA IF NOT EXISTS marble;

do $$
begin
   execute 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA marble TO ' || current_user;
end
$$;

DO $$
BEGIN
   EXECUTE 'ALTER DATABASE ' || current_database() || ' SET search_path TO marble, public';
END
$$;

DO $$
BEGIN
   EXECUTE format('ALTER ROLE %I SET search_path = marble, public;', current_user);
END
$$;

-- also set it for the current session
SET
  SEARCH_PATH = marble,
  public;

-- add UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS marble CASCADE;

-- +goose StatementEnd