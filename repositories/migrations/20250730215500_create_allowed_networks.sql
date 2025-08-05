-- +goose Up

alter table organizations
    add column allowed_networks cidr[];

-- +goose Down

alter table organizations
    drop column allowed_networks;
