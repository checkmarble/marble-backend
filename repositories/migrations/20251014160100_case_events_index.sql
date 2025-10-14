-- +goose Up
-- +goose NO TRANSACTION

create index concurrently idx_case_events_kinds
    on case_events (case_id, event_type, created_at desc, id desc);

-- +goose Down

drop index idx_case_events_kinds;
