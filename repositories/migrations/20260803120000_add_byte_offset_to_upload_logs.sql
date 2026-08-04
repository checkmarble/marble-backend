-- +goose Up

-- Byte offset of the first CSV row that has not been ingested yet, so a csv_ingestion job that
-- ran out of time can resume where it stopped instead of restarting the whole file.
-- bigint (int64) and not integer: the async upload path allows files up to 10 GiB, well past the
-- 2 GiB ceiling of int32.
alter table upload_logs
    add column byte_offset bigint not null default 0;

-- +goose Down

alter table upload_logs
    drop column byte_offset;
