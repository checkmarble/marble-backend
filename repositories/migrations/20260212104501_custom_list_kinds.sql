-- +goose Up

alter table custom_lists
    add column kind text;

update custom_lists
    set kind = 'text';

alter table custom_lists
    alter column kind set not null;

alter table custom_list_values
    alter column value drop not null,
    add column cidr inet;

-- +goose Down

alter table custom_lists
    drop column kind;

delete from custom_list_values
where cidr is not null;

alter table custom_list_values
    alter column value set not null,
    drop column ip;
