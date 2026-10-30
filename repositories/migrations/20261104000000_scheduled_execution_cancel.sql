-- +goose Up
-- +goose NO TRANSACTION

alter table scheduled_executions
    add column if not exists cancel_requested_at timestamptz,
    add column if not exists cancel_deadline     timestamptz;

drop index if exists unique_scheduled_per_scenario_idx;

create index concurrently unique_scheduled_per_scenario_idx on scheduled_executions (scenario_id)
    where status in ('pending', 'processing', 'cancelling');

-- +goose Down
-- +goose NO TRANSACTION

update scheduled_executions
    set status = 'failure', finished_at = coalesce(finished_at, now())
    where status in ('cancelling', 'cancelled');

drop index if exists unique_scheduled_per_scenario_idx;

create index concurrently unique_scheduled_per_scenario_idx on scheduled_executions (scenario_id)
    where status in ('pending', 'processing');

alter table scheduled_executions
    drop column cancel_requested_at,
    drop column cancel_deadline;
