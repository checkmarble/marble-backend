-- +goose Up
-- +goose StatementBegin
alter table sanction_check_configs
drop column score_modifier;

update sanction_check_configs
set
    forced_outcome = 'review'
where
    forced_outcome is null;

alter table sanction_check_configs
alter column forced_outcome
set not null;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
alter table sanction_check_configs
add column score_modifier integer default 0;

alter table sanction_check_configs
alter column forced_outcome
drop not null;

-- +goose StatementEnd