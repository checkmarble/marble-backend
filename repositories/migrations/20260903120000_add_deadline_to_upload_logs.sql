-- +goose Up

alter table upload_logs
    add column deadline_at timestamp with time zone;

-- +goose Down

alter table upload_logs
    drop column deadline_at;
