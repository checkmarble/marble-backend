-- +goose Up

alter table upload_logs
    add column deadline_at timestamp with time zone,
    add column error_code text;

update upload_logs
    set deadline_at = coalesce(finished_at, started_at + interval '12 hour')
    where deadline_at is null;

update upload_logs
    set error_code = case
        when coalesce(input_error, '') <> '' then 'invalid_input'
        else 'internal_error'
    end
    where status = 'failure' and error_code is null;

-- +goose Down

alter table upload_logs
    drop column error_code,
    drop column deadline_at;
