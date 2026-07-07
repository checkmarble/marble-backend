-- +goose Up

alter table upload_logs
    add column input_error text;

-- +goose Down

alter table upload_logs
    drop column input_error;
