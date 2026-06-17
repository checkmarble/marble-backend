-- +goose Up
-- Allow a table to have several (belongs_to) pivots pointing at different parent
-- tables (polymorphic / discriminated belongs_to: at most one applies per row).
-- The old index enforced a single live pivot per (organization_id, base_table_id).

drop index data_model_pivots_base_table_id_idx;

-- Lookup index for ListPivots (filters by organization_id [+ base_table_id]).
create index data_model_pivots_base_table_id_idx
on data_model_pivots (organization_id, base_table_id)
where deleted_at is null;

-- Prevent exact-duplicate live pivots while allowing distinct ones on the same
-- table. NULLS NOT DISTINCT (PG15+) makes two path pivots with the same
-- path_link_ids (field_id NULL) collide, while pivots with different paths or
-- fields coexist.
create unique index data_model_pivots_signature_idx
on data_model_pivots (organization_id, base_table_id, field_id, path_link_ids) nulls not distinct
where deleted_at is null;

-- +goose Down
drop index data_model_pivots_signature_idx;
drop index data_model_pivots_base_table_id_idx;

-- Restore the single-pivot-per-table constraint (fails if a table has >1 live pivot).
delete from data_model_pivots
where deleted_at is not null;

create unique index data_model_pivots_base_table_id_idx
on data_model_pivots (organization_id, base_table_id)
where deleted_at is null;
