-- +goose Up
-- +goose NO TRANSACTION

-- The dashboard aggregates lifecycle events across organizations, which the
-- org_id-leading audit indexes cannot serve. Both indexes only cover the three
-- small populations and are built without blocking audit writes.
create index concurrently idx_audit_dashboard_lifecycle
    on audit.audit_events ("table", created_at, entity_id)
    where "table" in ('organizations', 'tenants', 'users');

create index concurrently idx_audit_dashboard_creation
    on audit.audit_events ("table", entity_id, created_at)
    where operation = 'INSERT' and "table" in ('organizations', 'tenants', 'users');

-- +goose Down

drop index concurrently if exists audit.idx_audit_dashboard_lifecycle;
drop index concurrently if exists audit.idx_audit_dashboard_creation;
