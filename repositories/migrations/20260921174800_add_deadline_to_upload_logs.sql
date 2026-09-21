-- +goose Up

alter table upload_logs
    add column deadline_at timestamp with time zone;

update upload_logs
    set deadline_at = coalesce(finalized_at, started_at + interval '12 hour')
    where deadline_at is null;

-- +goose Down

alter table upload_logs
    drop column deadline_at;
