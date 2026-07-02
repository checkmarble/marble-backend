-- +goose Up
-- Lets a scheduled execution be processed from a manifest of object ids in blob storage,
-- walked by a single looping coordinator job.
--
-- manifest_blob_key       : blob key of the newline-delimited object-id manifest (NULL when not used).
-- manifest_byte_offset    : resume cursor into the manifest, always aligned to a line boundary.
-- manifest_rows_processed : number of object ids consumed from the manifest so far.
-- deadline                : wall-clock ceiling for the whole run.
alter table scheduled_executions
    add column manifest_blob_key       text,
    add column manifest_byte_offset    bigint      not null default 0,
    add column manifest_rows_processed bigint      not null default 0,
    add column deadline                timestamptz;

-- +goose Down

alter table scheduled_executions
    drop column manifest_blob_key,
    drop column manifest_byte_offset,
    drop column manifest_rows_processed,
    drop column deadline;
