-- +goose Up

alter table sanction_check_matches
    add column enriched bool default false;

-- +goose Down

alter table sanction_check_matches
    drop column enriched;
