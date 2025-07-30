-- +goose Up

alter table organizations
    add column whitelisted_subnets cidr[];

-- +goose Down

alter table organizations
    drop column whitelisted_subnets;
