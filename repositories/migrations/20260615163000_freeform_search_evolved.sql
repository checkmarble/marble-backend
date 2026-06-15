-- +goose Up
-- +goose StatementBegin
alter table screening_freeform_searches
add column search_config jsonb not null default '{}'::jsonb,
add column is_saved boolean not null default false,
add column result_hash bytea,
add column nb_hits integer not null default 0;

-- the format of the unstructured json we save for the search config has changed, and we'll need those for display, so we might as well start from a clean table
delete from screening_freeform_searches where true;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
alter table screening_freeform_searches
drop column search_config,
drop column is_saved,
drop column result_hash,
drop column nb_hits;

-- +goose StatementEnd
