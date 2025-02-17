-- +goose Up
-- +goose StatementBegin
alter table sanction_check_configs
drop column score_modifier;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
alter table sanction_check_configs
add column score_modifier integer default 0;

-- +goose StatementEnd