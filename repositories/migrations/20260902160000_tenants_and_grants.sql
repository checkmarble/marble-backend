-- +goose Up
-- +goose StatementBegin

CREATE TABLE tenants (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX tenants_name_unique_idx ON tenants (name)
WHERE deleted_at IS NULL;

-- Deleted organizations get deleted tenants so duplicate historical names do not
-- make the backfill fail, while every organization still gets exactly one tenant.
INSERT INTO tenants (id, name, created_at, deleted_at)
SELECT uuid_generate_v4(), name, now(), deleted_at
FROM organizations;

ALTER TABLE organizations ADD COLUMN tenant_id uuid;

WITH numbered_organizations AS (
    SELECT id, name, deleted_at,
           row_number() OVER (PARTITION BY name, deleted_at ORDER BY id) AS row_number
    FROM organizations
), numbered_tenants AS (
    SELECT id, name, deleted_at,
           row_number() OVER (PARTITION BY name, deleted_at ORDER BY id) AS row_number
    FROM tenants
)
UPDATE organizations o
SET tenant_id = t.id
FROM numbered_organizations no
JOIN numbered_tenants t
  ON t.name = no.name
 AND t.deleted_at IS NOT DISTINCT FROM no.deleted_at
 AND t.row_number = no.row_number
WHERE o.id = no.id;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM organizations WHERE tenant_id IS NULL) THEN
        RAISE EXCEPTION 'organization backfill left orphan organizations';
    END IF;
END
$$;

ALTER TABLE organizations
    ALTER COLUMN tenant_id SET NOT NULL,
    ADD CONSTRAINT organizations_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id);

CREATE TABLE grants (
    id uuid PRIMARY KEY,
    principal_type text NOT NULL,
    principal_id text NOT NULL,
    principal_authority text NOT NULL,
    tenant_id uuid REFERENCES tenants(id),
    organization_id uuid REFERENCES organizations(id),
    role text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    revoked_at timestamptz,
    CONSTRAINT grants_principal_type_check
        CHECK (principal_type IN ('user', 'api_key')),
    CONSTRAINT grants_scope_check
        CHECK (num_nonnulls(tenant_id, organization_id) <= 1),
    CONSTRAINT grants_api_key_scope_check
        CHECK (principal_type <> 'api_key' OR organization_id IS NOT NULL)
);

CREATE UNIQUE INDEX grants_platform_uniq ON grants
    (principal_type, principal_id, principal_authority, role)
WHERE tenant_id IS NULL AND organization_id IS NULL AND revoked_at IS NULL;

CREATE UNIQUE INDEX grants_tenant_uniq ON grants
    (principal_type, principal_id, principal_authority, tenant_id, role)
WHERE tenant_id IS NOT NULL AND revoked_at IS NULL;

CREATE UNIQUE INDEX grants_organization_uniq ON grants
    (principal_type, principal_id, principal_authority, organization_id, role)
WHERE organization_id IS NOT NULL AND revoked_at IS NULL;

CREATE VIEW active_grants AS
SELECT *
FROM grants
WHERE revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

INSERT INTO grants (id, principal_type, principal_id, principal_authority, tenant_id, role)
SELECT uuid_generate_v4(), 'user', u.id::text, 'marble', o.tenant_id, 'TENANT_ADMIN'
FROM users u
JOIN organizations o ON o.id = u.organization_id
WHERE u.organization_id IS NOT NULL
  AND u.organization_id <> '00000000-0000-0000-0000-000000000000'
  AND u.deleted_at IS NULL
  AND u.role = 4;

ALTER TABLE audit.audit_events ADD COLUMN tenant_id uuid;

CREATE OR REPLACE FUNCTION global_audit() RETURNS trigger AS $$
DECLARE
    actor_user_id text := nullif(current_setting('custom.current_user_id', true), '');
    actor_api_key_id uuid := nullif(current_setting('custom.current_api_key_id', true), '')::uuid;
    scope_org_id uuid := nullif(current_setting('custom.current_org_id', true), '')::uuid;
    scope_tenant_id uuid := nullif(current_setting('custom.current_tenant_id', true), '')::uuid;
BEGIN
    IF actor_user_id IS NULL AND actor_api_key_id IS NULL THEN
        RETURN NULL;
    END IF;

    IF TG_OP = 'DELETE' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, created_at)
        VALUES ('DELETE', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, OLD.id, to_jsonb(OLD), now());
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, previous_data, created_at)
        VALUES ('UPDATE', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, NEW.id, to_jsonb(NEW), to_jsonb(OLD), now());
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, created_at)
        VALUES ('INSERT', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER audit
AFTER INSERT OR UPDATE OR DELETE ON organizations
FOR EACH ROW EXECUTE FUNCTION global_audit();

CREATE TRIGGER grants_audit
AFTER INSERT OR UPDATE OR DELETE ON grants
FOR EACH ROW EXECUTE FUNCTION global_audit();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS grants_audit ON grants;
DROP VIEW active_grants;
DROP TABLE grants;

DROP TRIGGER IF EXISTS audit ON organizations;
CREATE TRIGGER audit
AFTER UPDATE OF allowed_networks ON organizations
FOR EACH ROW EXECUTE FUNCTION global_audit();

CREATE OR REPLACE FUNCTION global_audit() RETURNS trigger AS $$
BEGIN
    IF current_setting('custom.current_user_id', true) IS NULL
       AND current_setting('custom.current_api_key_id', true) IS NULL THEN
        RETURN NULL;
    END IF;

    IF TG_OP = 'DELETE' THEN
        INSERT INTO audit.audit_events (operation, org_id, user_id, api_key_id, "table", entity_id, data, created_at)
        VALUES ('DELETE', nullif(current_setting('custom.current_org_id', true), '')::uuid,
                nullif(current_setting('custom.current_user_id', true), ''),
                nullif(current_setting('custom.current_api_key_id', true), '')::uuid,
                TG_TABLE_NAME, OLD.id, to_jsonb(OLD), now());
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO audit.audit_events (operation, org_id, user_id, api_key_id, "table", entity_id, data, previous_data, created_at)
        VALUES ('UPDATE', nullif(current_setting('custom.current_org_id', true), '')::uuid,
                nullif(current_setting('custom.current_user_id', true), ''),
                nullif(current_setting('custom.current_api_key_id', true), '')::uuid,
                TG_TABLE_NAME, NEW.id, to_jsonb(NEW), to_jsonb(OLD), now());
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO audit.audit_events (operation, org_id, user_id, api_key_id, "table", entity_id, data, created_at)
        VALUES ('INSERT', nullif(current_setting('custom.current_org_id', true), '')::uuid,
                nullif(current_setting('custom.current_user_id', true), ''),
                nullif(current_setting('custom.current_api_key_id', true), '')::uuid,
                TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
ALTER TABLE audit.audit_events DROP COLUMN tenant_id;
ALTER TABLE organizations DROP CONSTRAINT organizations_tenant_id_fkey;
ALTER TABLE organizations DROP COLUMN tenant_id;
DROP TABLE tenants;

-- +goose StatementEnd
