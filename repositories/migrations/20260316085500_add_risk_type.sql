-- +goose Up

alter table scoring_rules
    add column risk_type text default 'other';

-- +goose Down

alter table scoring_rules
    drop column risk_type;
