-- +goose Up
-- +goose NO TRANSACTION
-- +goose StatementBegin

alter table case_events
    add column org_id uuid;

update case_events
set org_id = c.org_id
from cases c
where c.id = case_events.case_id;

alter table case_events
    alter column org_id set not null;

-- +goose StatementEnd

create index concurrently if not exists idx_case_events_by_org
    on case_events (org_id, event_type, created_at)
    where event_type in ('outcome_updated');

-- +goose Down

drop index idx_case_events_by_org;
alter table case_events drop column org_id;
