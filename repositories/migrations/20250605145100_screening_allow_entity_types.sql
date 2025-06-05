-- +goose Up

alter table sanction_check_configs
    add column entity_type text not null default 'Thing';

-- +goose Down

alter table sanction_check_configs
    drop column entity_type;
