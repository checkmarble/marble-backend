-- +goose Up
-- +goose StatementBegin
-- create the analytics schema and an analytics user
CREATE SCHEMA IF NOT EXISTS analytics;

do $$
begin
   execute 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA analytics TO ' || current_user;
end
$$;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS analytics CASCADE;

-- +goose StatementEnd