-- +goose Up

alter table rule_snoozes
    alter column created_by_user drop not null;

-- +goose Down

alter table rule_snoozes
    alter column created_by_user set not null;