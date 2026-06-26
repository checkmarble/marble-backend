-- +goose Up
-- Scalable batch execution v2: a scheduled execution is driven by a single looping
-- coordinator that walks a GCS manifest of object ids instead of fanning out one
-- river job + one decisions_to_create row per object.
--
-- manifest_blob_key      : GCS key of the newline-delimited object-id manifest (NULL for the legacy path).
-- manifest_byte_offset   : resume cursor into the manifest, always aligned to a line boundary.
-- manifest_rows_processed : number of object ids consumed from the manifest so far.
-- deadline               : wall-clock ceiling for the whole run; the only termination on sustained retryable failure.
alter table scheduled_executions
    add column manifest_blob_key       text,
    add column manifest_byte_offset    bigint      not null default 0,
    add column manifest_rows_processed bigint      not null default 0,
    add column deadline                timestamptz;

-- Lightweight sidecar for hard failures. The coordinator advances in-order and stops
-- on a hard failure, so this stays small. No FK so deleting an execution never blocks
-- on it and the type stays decoupled.
create table scheduled_execution_failures (
    id                     uuid        primary key default gen_random_uuid(),
    scheduled_execution_id uuid        not null,
    object_id              text        not null,
    error                  text        not null,
    created_at             timestamptz not null default now()
);

create index scheduled_execution_failures_exec_idx
    on scheduled_execution_failures (scheduled_execution_id);

-- +goose Down
drop table scheduled_execution_failures;

alter table scheduled_executions
    drop column manifest_blob_key,
    drop column manifest_byte_offset,
    drop column manifest_rows_processed,
    drop column deadline;
