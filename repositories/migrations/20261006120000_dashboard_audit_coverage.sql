-- +goose Up
-- +goose StatementBegin

-- Writers wait while the triggers are swapped, readers do not. Holding the write
-- side of every audited population until commit guarantees that no lifecycle
-- change can land between the coverage start and the new triggers. Fail fast
-- rather than queue all writes behind a long-running transaction.
SET LOCAL lock_timeout = '10s';
LOCK TABLE organizations, tenants, users, audit.audit_events IN SHARE ROW EXCLUSIVE MODE;

-- Lifecycle capture for the backoffice dashboard. Unlike global_audit, events are
-- recorded without an end-user or API-key actor (system, imports, direct SQL).
-- Scope is copied from the session only for actor-driven events, exactly like
-- global_audit, so organization-scoped audit consumers see the same events as
-- before; actorless events stay unscoped and only feed global aggregates.
-- clock_timestamp() orders events after the coverage start even for transactions
-- that began before this migration, and keeps repeated transitions inside one
-- transaction distinguishable.
CREATE FUNCTION dashboard_entity_audit() RETURNS trigger AS $$
DECLARE
    actor_user_id text := nullif(current_setting('custom.current_user_id', true), '');
    actor_api_key_id uuid := nullif(current_setting('custom.current_api_key_id', true), '')::uuid;
    scope_org_id uuid;
    scope_tenant_id uuid;
BEGIN
    IF actor_user_id IS NOT NULL OR actor_api_key_id IS NOT NULL THEN
        scope_org_id := nullif(current_setting('custom.current_org_id', true), '')::uuid;
        scope_tenant_id := nullif(current_setting('custom.current_tenant_id', true), '')::uuid;
    END IF;

    IF TG_OP = 'DELETE' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, created_at)
        VALUES ('DELETE', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, OLD.id, to_jsonb(OLD), clock_timestamp());
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, previous_data, created_at)
        VALUES ('UPDATE', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, NEW.id, to_jsonb(NEW), to_jsonb(OLD), clock_timestamp());
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, created_at)
        VALUES ('INSERT', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, NEW.id, to_jsonb(NEW), clock_timestamp());
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- api_keys duplicated every users event through global_audit.
DROP TRIGGER IF EXISTS api_keys ON users;
CREATE OR REPLACE TRIGGER audit AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION dashboard_entity_audit();
CREATE OR REPLACE TRIGGER audit AFTER INSERT OR UPDATE OR DELETE ON organizations
FOR EACH ROW EXECUTE FUNCTION dashboard_entity_audit();
CREATE TRIGGER audit AFTER INSERT OR UPDATE OR DELETE ON tenants
FOR EACH ROW EXECUTE FUNCTION dashboard_entity_audit();

-- Earliest instant from which every lifecycle event ('all') or every creation
-- event ('new') of an entity is known to be retained. Starts at deployment:
-- older events exist but cannot prove completeness.
CREATE TABLE audit.dashboard_coverage (
    entity text NOT NULL CHECK (entity IN ('organizations', 'tenants', 'users')),
    measure text NOT NULL CHECK (measure IN ('all', 'new')),
    since timestamp(6) with time zone NOT NULL,
    PRIMARY KEY (entity, measure)
);

-- Losing or rewriting an event at instant T invalidates history up to T only:
-- every interval strictly after T still has all of its events. Coverage never
-- moves backwards.
CREATE FUNCTION dashboard_lose_coverage(lost_entity text, lost_creation boolean, lost_at timestamp with time zone)
RETURNS void AS $$
    UPDATE audit.dashboard_coverage
    SET since = lost_at + interval '1 microsecond'
    WHERE entity = lost_entity
      AND (measure = 'all' OR lost_creation)
      AND since <= lost_at;
$$ LANGUAGE sql;

CREATE FUNCTION dashboard_audit_event_lost() RETURNS trigger AS $$
BEGIN
    PERFORM dashboard_lose_coverage(OLD."table", OLD.operation = 'INSERT', OLD.created_at);
    IF TG_OP = 'UPDATE' THEN
        PERFORM dashboard_lose_coverage(NEW."table", NEW.operation = 'INSERT', NEW.created_at);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Row triggers filtered by WHEN: pruning or editing events of any other table
-- (decisions, cases, ...) never queues a trigger and never touches coverage.
CREATE TRIGGER dashboard_coverage_on_delete AFTER DELETE ON audit.audit_events
FOR EACH ROW WHEN (OLD."table" IN ('organizations', 'tenants', 'users'))
EXECUTE FUNCTION dashboard_audit_event_lost();
CREATE TRIGGER dashboard_coverage_on_update AFTER UPDATE ON audit.audit_events
FOR EACH ROW WHEN (
    (OLD."table" IN ('organizations', 'tenants', 'users') OR NEW."table" IN ('organizations', 'tenants', 'users'))
    AND (OLD."table", OLD.entity_id, OLD.operation, OLD.data, OLD.previous_data, OLD.created_at)
        IS DISTINCT FROM (NEW."table", NEW.entity_id, NEW.operation, NEW.data, NEW.previous_data, NEW.created_at)
)
EXECUTE FUNCTION dashboard_audit_event_lost();

-- TRUNCATE emits no row events. On the audit store every event is gone; on a
-- population, rows vanish without lifecycle events while recorded creations stay
-- valid.
CREATE FUNCTION dashboard_truncated() RETURNS trigger AS $$
BEGIN
    UPDATE audit.dashboard_coverage
    SET since = greatest(since, clock_timestamp())
    WHERE TG_TABLE_SCHEMA = 'audit' OR (entity = TG_TABLE_NAME AND measure = 'all');
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER dashboard_coverage_on_truncate AFTER TRUNCATE ON audit.audit_events
FOR EACH STATEMENT EXECUTE FUNCTION dashboard_truncated();
CREATE TRIGGER dashboard_coverage_on_truncate AFTER TRUNCATE ON organizations
FOR EACH STATEMENT EXECUTE FUNCTION dashboard_truncated();
CREATE TRIGGER dashboard_coverage_on_truncate AFTER TRUNCATE ON tenants
FOR EACH STATEMENT EXECUTE FUNCTION dashboard_truncated();
CREATE TRIGGER dashboard_coverage_on_truncate AFTER TRUNCATE ON users
FOR EACH STATEMENT EXECUTE FUNCTION dashboard_truncated();

INSERT INTO audit.dashboard_coverage (entity, measure, since)
SELECT entity, measure, deployment.since
FROM (SELECT clock_timestamp() AS since) deployment,
    unnest(ARRAY['organizations', 'tenants', 'users']) entity,
    unnest(ARRAY['all', 'new']) measure;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Every audit event captured meanwhile (tenants, actorless lifecycle events) is kept.
SET LOCAL lock_timeout = '10s';
LOCK TABLE organizations, tenants, users, audit.audit_events IN SHARE ROW EXCLUSIVE MODE;

DROP TRIGGER dashboard_coverage_on_truncate ON users;
DROP TRIGGER dashboard_coverage_on_truncate ON tenants;
DROP TRIGGER dashboard_coverage_on_truncate ON organizations;
DROP TRIGGER dashboard_coverage_on_truncate ON audit.audit_events;
DROP TRIGGER dashboard_coverage_on_update ON audit.audit_events;
DROP TRIGGER dashboard_coverage_on_delete ON audit.audit_events;
DROP FUNCTION dashboard_truncated();
DROP FUNCTION dashboard_audit_event_lost();
DROP FUNCTION dashboard_lose_coverage(text, boolean, timestamp with time zone);
DROP TABLE audit.dashboard_coverage;

DROP TRIGGER audit ON tenants;
CREATE OR REPLACE TRIGGER audit AFTER INSERT OR UPDATE OR DELETE ON organizations
FOR EACH ROW EXECUTE FUNCTION global_audit();
CREATE OR REPLACE TRIGGER audit AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION global_audit();
-- Restore the previous schema faithfully, including the duplicate users trigger.
CREATE TRIGGER api_keys AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION global_audit();
DROP FUNCTION dashboard_entity_audit();

-- +goose StatementEnd
