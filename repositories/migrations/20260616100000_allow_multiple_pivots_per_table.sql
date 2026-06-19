-- +goose Up
-- Allow a table to have several (belongs_to) pivots pointing at different parent
-- tables (polymorphic / discriminated belongs_to: at most one applies per row).
-- The old index enforced a single live pivot per (organization_id, base_table_id).

drop index data_model_pivots_base_table_id_idx;

-- Prevent exact-duplicate live pivots while allowing distinct ones on the same
-- table. NULLS NOT DISTINCT (PG15+) makes two path pivots with the same
-- path_link_ids (field_id NULL) collide, while pivots with different paths or
-- fields coexist. Its (organization_id, base_table_id) prefix also serves the
-- ListPivots lookups, so no separate lookup index is needed.
create unique index data_model_pivots_signature_idx
on data_model_pivots (organization_id, base_table_id, field_id, path_link_ids) nulls not distinct
where deleted_at is null;

-- +goose Down
drop index data_model_pivots_signature_idx;

-- The restored index allows only one live pivot per (organization_id, base_table_id),
-- but the forward migration may have created several. Soft-delete all but the oldest
-- live pivot per table so the unique index can be recreated. Soft-delete (rather than
-- hard delete) preserves decisions that reference these pivots by id.
update data_model_pivots p
set deleted_at = now()
where p.deleted_at is null
  and exists (
    select 1
    from data_model_pivots q
    where q.organization_id = p.organization_id
      and q.base_table_id = p.base_table_id
      and q.deleted_at is null
      and (q.created_at, q.id) < (p.created_at, p.id)
  );

create unique index data_model_pivots_base_table_id_idx
on data_model_pivots (organization_id, base_table_id)
where deleted_at is null;
