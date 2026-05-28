-- +goose Up
alter table screenings
drop column whitelisted_entities,
drop column search_datasets;

-- +goose Down
alter table screenings
add column whitelisted_entities text[],
add column search_datasets text[];