-- +goose Up
-- +goose StatementBegin
-- create and make default the marble schema
CREATE SCHEMA marble;

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA marble TO postgres;

ALTER DATABASE marble
SET search_path TO marble,
  public;

ALTER ROLE postgres
SET search_path TO marble,
  public;

-- also set it for the current session
SET SEARCH_PATH=marble,public;

-- add UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS marble CASCADE;

-- +goose StatementEnd