-- +goose Up
-- +goose NO TRANSACTION

create index concurrently if not exists idx_analytics_export
on decisions (org_id, trigger_object_type, created_at desc, id desc);

-- +goose Down

drop index idx_analytics_export;
