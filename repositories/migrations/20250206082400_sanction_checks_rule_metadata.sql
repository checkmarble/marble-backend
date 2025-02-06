-- +goose Up
-- +goose StatementBegin

alter table sanction_check_configs
    add column name text not null,
    add column description text not null,
    add column rule_group varchar(255) not null default '',
    alter column trigger_rule drop not null,
    alter column query drop not null;

-- +goose StatementEnd

-- +goose Down

alter table sancion_checks_configs
    drop column name,
    drop column description,
    drop column rule_group,
    alter column trigger_rule set not null,
    alter column query set not null;