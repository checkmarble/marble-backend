-- +goose Up

alter table sanction_check_configs
    add column threshold integer null default null;

-- +goose Down

alter table sanction_check_configs
    drop column threshold;
