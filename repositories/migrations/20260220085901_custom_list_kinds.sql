-- +goose Up

alter table custom_lists
    add column kind text not null default 'text';

alter table custom_list_values
    alter column value drop not null,
    add column cidr inet;

-- +goose Down

delete from custom_lists
where kind = 'cidrs';

delete from custom_list_values
where cidr is not null;

alter table custom_lists
    drop column kind;

alter table custom_list_values
    alter column value set not null,
    drop column cidr;
