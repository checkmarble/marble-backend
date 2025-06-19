-- +goose Up

alter table sanction_checks
    add column initial_query jsonb;

-- +goose Down

alter table sanction_checks
    drop column initial_query;
