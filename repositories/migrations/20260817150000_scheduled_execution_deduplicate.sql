-- +goose Up
-- Effective dedup setting for this run, resolved once at creation from
-- scenarios.deduplicate_batch_objects (the default) plus any manual override.
--
-- Snapshotted rather than read live from the scenario because the coordinator re-enters
-- Run() -- and therefore loadInvariants() -- on every River slice (JobSnooze every ~9min),
-- and a full-table manifest spans several slices. Reading the scenario each slice means a
-- mid-run toggle changes behaviour between slices: an ON->OFF flip makes the remainder of
-- the run stop consulting *and* stop writing claims, producing duplicate decisions on that
-- portion and leaving those objects unclaimed for the next run. The guarantee would break
-- silently for part of the run.
alter table scheduled_executions add column deduplicate_objects boolean not null default false;

-- +goose Down
alter table scheduled_executions drop column deduplicate_objects;
