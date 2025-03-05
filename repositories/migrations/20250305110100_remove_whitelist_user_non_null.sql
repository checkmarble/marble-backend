-- +goose Up

alter table sanction_check_whitelists
    alter column whitelisted_by drop not null;

-- +goose Down

alter table sanction_check_whietlists
    alter column whitelisted_by set not null;
