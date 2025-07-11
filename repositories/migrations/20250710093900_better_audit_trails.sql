-- +goose Up
-- +goose StatementBegin

alter table "audit"."audit_events"
    add column if not exists org_id uuid,
    add column if not exists api_key_id uuid,
    add column if not exists previous_data jsonb;

update "audit"."audit_events" events
set org_id = sq.organization_id
from ( select id, organization_id from users ) sq
where sq.id::text = events.user_id;

create or replace function global_audit() returns trigger as $$
begin
    if current_setting('custom.current_user_id', true) is null and current_setting('custom.current_api_key_id', true) is null then
        return null;
    end if;

    if (TG_OP = 'DELETE') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "api_key_id", "table", "entity_id", "data", "created_at")
        values ('DELETE', current_setting('custom.current_org_id', true)::uuid, current_setting('custom.current_user_id', true), current_setting('custom.current_api_key_id', true)::uuid, TG_TABLE_NAME, old.id, to_jsonb(OLD), now());
    elsif (TG_OP = 'UPDATE') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "api_key_id", "table", "entity_id", "data", "previous_data", "created_at")
        values ('UPDATE', current_setting('custom.current_org_id', true)::uuid, current_setting('custom.current_user_id', true), current_setting('custom.current_api_key_id', true)::uuid, TG_TABLE_NAME, new.id, to_jsonb(NEW), to_jsonb(OLD), now());
    elsif (TG_OP = 'INSERT') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "api_key_id", "table", "entity_id", "data", "created_at")
        values ('INSERT', current_setting('custom.current_org_id', true)::uuid, current_setting('custom.current_user_id', true), current_setting('custom.current_api_key_id', true)::uuid, TG_TABLE_NAME, new.id, to_jsonb(NEW), now());
    end if;
    return null;
end;
$$ language plpgsql;

-- Any action on users or permissions

create or replace trigger audit
after insert or update or delete
on users
for each row execute function global_audit();

-- Any action on API keys

create or replace trigger api_keys
after insert or update or delete
on users
for each row execute function global_audit();

-- Any status or outcome change on cases

create or replace trigger audit
after update
on cases
for each row when (
    old.outcome != new.outcome
)
execute function global_audit();

-- Any review status change on decisions

create or replace trigger audit
after update
on decisions
for each row when (
    old.review_status != new.review_status
)
execute function global_audit();

-- Any change in review status for screenings

create or replace trigger audit
after update
on sanction_checks
for each row when (
    old.status != new.status
)
execute function global_audit();

create or replace trigger audit
after update
on sanction_check_matches
for each row when (
    old.status != new.status
)
execute function global_audit();

-- Any commit of a scenario iteration

create or replace trigger audit
after update
on scenario_iterations
for each row when (
    old.version is null and
    new.version is not null
)
execute function global_audit();

-- Any scenario publication

create or replace trigger audit
after insert
on scenario_publications
for each row when (
    new.publication_action in ('publish', 'unpublish')
)
execute function global_audit();

-- Any snoozing of rules

create or replace trigger audit
after insert
on rule_snoozes
for each row
execute function global_audit();

-- Any screening whitelist creation

create or replace trigger audit
after insert
on sanction_check_whitelists
for each row
execute function global_audit();

-- +goose StatementEnd

-- +goose Down
