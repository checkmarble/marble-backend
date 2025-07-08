-- +goose Up
-- +goose StatementBegin

alter table "audit"."audit_events"
    add column if not exists org_id uuid;

update "audit"."audit_events" events
set org_id = sq.organization_id
from ( select id, organization_id from users ) sq
where sq.id::text = events.user_id;

create or replace function global_audit() returns trigger as $$
begin
    if (TG_OP = 'DELETE') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "table", "entity_id", "data", "created_at")
        values ('DELETE', current_setting('custom.current_org_id', TRUE)::uuid, current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, OLD.id, to_jsonb(OLD), now());
    elsif (TG_OP = 'UPDATE') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "table", "entity_id", "data", "created_at")
        values ('UPDATE', current_setting('custom.current_org_id', TRUE)::uuid, current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());
    elsif (TG_OP = 'INSERT') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "table", "entity_id", "data", "created_at")
        values ('INSERT', current_setting('custom.current_org_id', TRUE)::uuid, current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());
    end if;
    return null;
end;
$$ language plpgsql;

-- +goose StatementEnd

-- +goose Down
