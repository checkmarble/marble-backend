-- +goose Up
-- +goose StatementBegin

create or replace function global_audit() returns trigger as $$
begin
    if (nullif(current_setting('custom.current_user_id', true), '') is null and nullif(current_setting('custom.current_api_key_id', true), '') is null) or nullif(current_setting('custom.current_org_id', true), '') is null then
        return null;
    end if;

    if (TG_OP = 'DELETE') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "api_key_id", "table", "entity_id", "data", "created_at")
        values ('DELETE', current_setting('custom.current_org_id', true)::uuid, nullif(current_setting('custom.current_user_id', true), ''), nullif(current_setting('custom.current_api_key_id', true), '')::uuid, TG_TABLE_NAME, old.id, to_jsonb(OLD), now());
    elsif (TG_OP = 'UPDATE') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "api_key_id", "table", "entity_id", "data", "previous_data", "created_at")
        values ('UPDATE', current_setting('custom.current_org_id', true)::uuid, nullif(current_setting('custom.current_user_id', true), ''), nullif(current_setting('custom.current_api_key_id', true), '')::uuid, TG_TABLE_NAME, new.id, to_jsonb(NEW), to_jsonb(OLD), now());
    elsif (TG_OP = 'INSERT') then
        insert into audit.audit_events ("operation", "org_id", "user_id", "api_key_id", "table", "entity_id", "data", "created_at")
        values ('INSERT', current_setting('custom.current_org_id', true)::uuid, nullif(current_setting('custom.current_user_id', true), ''), nullif(current_setting('custom.current_api_key_id', true), '')::uuid, TG_TABLE_NAME, new.id, to_jsonb(NEW), now());
    end if;
    return null;
end;
$$ language plpgsql;

-- +goose StatementEnd

-- +goose Down
