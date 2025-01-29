-- +goose Up

alter table sanction_check_configs add column query jsonb not null default '{}';

-- +goose Down

alter table sanction_check_configs drop column query;