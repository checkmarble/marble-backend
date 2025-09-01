-- +goose Up

alter table upload_logs
    add column error text null;

-- +goose Down

alter table upload_logs
    drop column error;
