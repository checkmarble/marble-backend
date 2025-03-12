-- +goose Up

alter table cases
    add column snoozed_until timestamp with time zone null;

-- +goose Down

alter table cases
    drop column snoozed_until;