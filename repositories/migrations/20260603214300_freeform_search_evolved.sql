-- +goose Up
-- +goose StatementBegin
alter table screening_freeform_searches
add column search_config jsonb not null default '{}'::jsonb;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
alter table screening_freeform_searches
drop column search_config;

-- +goose StatementEnd