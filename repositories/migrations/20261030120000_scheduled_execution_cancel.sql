-- +goose Up

alter table scheduled_executions
    add column cancel_requested_at timestamptz,
    add column cancel_deadline     timestamptz;

drop index if exists unique_scheduled_per_scenario_idx;

create index unique_scheduled_per_scenario_idx on scheduled_executions (scenario_id)
    where status in ('pending', 'processing', 'cancelling');

-- +goose Down

drop index if exists unique_scheduled_per_scenario_idx;

create index unique_scheduled_per_scenario_idx on scheduled_executions (scenario_id)
    where status in ('pending', 'processing');

alter table scheduled_executions
    drop column cancel_requested_at,
    drop column cancel_deadline;
