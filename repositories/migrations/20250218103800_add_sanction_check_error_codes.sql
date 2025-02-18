-- +goose Up

alter table sanction_checks
    add column error_codes text[];

-- +goose Down

alter table sanction_checks
    drop column error_codes;