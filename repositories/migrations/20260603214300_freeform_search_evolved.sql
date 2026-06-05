-- +goose Up
-- +goose StatementBegin
alter table screening_freeform_searches
add column search_config jsonb not null default '{}'::jsonb,
add column is_saved boolean not null default false,
add column result_hash bytea;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
alter table screening_freeform_searches
drop column search_config,
drop column is_saved,
drop column result_hash;

-- +goose StatementEnd