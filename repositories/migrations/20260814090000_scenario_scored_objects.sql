-- +goose Up
alter table scenarios add column deduplicate_batch_objects boolean not null default false;

-- Enforces, for batch executions only, at most one decision per (scenario, data object).
--
-- This cannot be an index on `decisions`: that table has no object_id column (only
-- trigger_object jsonb) and, more decisively, duplicates already exist there -- creating
-- decisions twice via overlapping batch windows is precisely the workaround this feature
-- replaces, so a unique index on existing data would fail to build.
--
-- The unique index below IS the guarantee. It holds against any writer, at any isolation
-- level, including concurrent runs of the same scenario (the pending/processing guard in
-- schedule_scenarios.go is read outside a transaction, and unique_scheduled_per_scenario_idx
-- is not actually UNIQUE despite its name).
--
-- Rows are never deleted by the application. `created_at` exists so support can unblock a
-- scenario after a faulty trigger publication, which otherwise excludes those objects
-- permanently:
--   delete from scenario_scored_objects where scenario_id = '...' and created_at >= '...';
create table scenario_scored_objects (
    scenario_id uuid not null references scenarios(id) on delete cascade,
    object_id   text not null,
    created_at  timestamptz not null default now()
);

-- No org_id: reachable through scenario_id, which is globally unique. No table_name: a
-- scenario has exactly one trigger_object_type.
create unique index scenario_scored_objects_unique_idx
    on scenario_scored_objects (scenario_id, object_id);

-- +goose Down
drop table scenario_scored_objects;
alter table scenarios drop column deduplicate_batch_objects;
