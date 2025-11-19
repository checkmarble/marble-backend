-- +goose Up
-- +goose NO TRANSACTION

create index concurrently idx_audit_created_at
    on audit.audit_events (org_id, created_at, id);

create index concurrently idx_audit_entity_id
    on audit.audit_events (org_id, entity_id, created_at, id);

create index concurrently idx_audit_user_id
    on audit.audit_events (org_id, user_id, created_at, id)
    where user_id is not null;

create index concurrently idx_audit_api_key_id
    on audit.audit_events (org_id, api_key_id, created_at, id)
    where api_key_id is not null;

-- +goose Down

drop index audit.idx_audit_created_at;
drop index audit.idx_audit_entity_id;
drop index audit.idx_audit_user_id;
drop index audit.idx_audit_api_key_id;
