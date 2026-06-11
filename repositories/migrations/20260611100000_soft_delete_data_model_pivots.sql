-- +goose Up
alter table data_model_pivots
add column deleted_at timestamp with time zone;

-- Replace the unique index with a partial one so that a new pivot can be created
-- on a table after its previous pivot has been soft-deleted.
drop index data_model_pivots_base_table_id_idx;

create unique index data_model_pivots_base_table_id_idx
on data_model_pivots (organization_id, base_table_id)
where deleted_at is null;

-- +goose Down
delete from data_model_pivots
where deleted_at is not null;

drop index data_model_pivots_base_table_id_idx;

create unique index data_model_pivots_base_table_id_idx
on data_model_pivots (organization_id, base_table_id);

alter table data_model_pivots
drop column deleted_at;
