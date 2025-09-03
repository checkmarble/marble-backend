-- +goose Up
-- +goose StatementBegin
alter table decision_rules
add column created_at timestamp with time zone not null default now();

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
alter table decision_rules
drop column created_at;

-- +goose StatementEnd