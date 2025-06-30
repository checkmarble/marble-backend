-- +goose Up

alter table organizations
    alter column sanctions_limit set default 10;

-- +goose Down
