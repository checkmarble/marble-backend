-- +goose Up

alter table upload_logs
    add column byte_offset bigint not null default 0;

-- +goose Down

alter table upload_logs
    drop column byte_offset;
