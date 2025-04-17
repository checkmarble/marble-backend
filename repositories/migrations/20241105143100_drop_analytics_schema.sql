-- +goose Up
-- +goose StatementBegin
DROP SCHEMA IF EXISTS analytics CASCADE;

DROP USER IF EXISTS analytics;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS analytics;

do $$
begin
   execute 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA analytics TO ' || current_user;
end
$$;

-- +goose StatementEnd