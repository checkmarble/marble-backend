-- +goose Up
-- +goose NO TRANSACTION

create index concurrently if not exists idx_case_events_by_org
    on case_events (org_id, event_type, created_at)
    where event_type in ('outcome_updated');

-- +goose Down

drop index idx_case_events_by_org;
