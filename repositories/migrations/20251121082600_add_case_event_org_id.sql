-- +goose Up

alter table case_events
    add column org_id uuid not null default '00000000-0000-0000-0000-000000000000';

update case_events
set org_id = c.org_id
from cases c
where c.id = case_events.case_id;

-- +goose Down

alter table case_events drop column org_id;
