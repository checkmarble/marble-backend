-- +goose Up
-- +goose StatementBegin

alter table scenario_test_run
    add column updated_at timestamp with time zone null default now();

update scenario_test_run
set updated_at = now();

alter table scenario_test_run
    alter column updated_at set not null;

-- +goose StatementEnd

-- +goose Down

alter table scenario_test_run
    drop column updated_at;