-- +goose Up
-- +goose StatementBegin

CREATE TABLE tenant_oidc_configurations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL UNIQUE REFERENCES tenants(id),
    issuer text NOT NULL,
    client_id text NOT NULL,
    client_secret bytea NOT NULL,
    scopes text[] NOT NULL DEFAULT '{}',
    extra_params jsonb NOT NULL DEFAULT '{}',
    email_claim text NOT NULL DEFAULT '',
    sso_enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tenant_domains (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    domain text NOT NULL,
    verified_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tenant_domains_verified_domain_unique_idx ON tenant_domains (domain)
WHERE verified_at IS NOT NULL;

CREATE OR REPLACE FUNCTION global_audit() RETURNS trigger AS $$
DECLARE
    actor_user_id text := nullif(current_setting('custom.current_user_id', true), '');
    actor_api_key_id uuid := nullif(current_setting('custom.current_api_key_id', true), '')::uuid;
    scope_org_id uuid := nullif(current_setting('custom.current_org_id', true), '')::uuid;
    scope_tenant_id uuid := nullif(current_setting('custom.current_tenant_id', true), '')::uuid;
    new_data jsonb;
    old_data jsonb;
BEGIN
    IF actor_user_id IS NULL AND actor_api_key_id IS NULL THEN
        RETURN NULL;
    END IF;

    IF TG_OP <> 'DELETE' THEN
        new_data := to_jsonb(NEW);
    END IF;
    IF TG_OP <> 'INSERT' THEN
        old_data := to_jsonb(OLD);
    END IF;
    IF TG_TABLE_NAME = 'tenant_oidc_configurations' THEN
        new_data := new_data - 'client_secret';
        old_data := old_data - 'client_secret';
    END IF;

    IF TG_OP = 'DELETE' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, created_at)
        VALUES ('DELETE', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, OLD.id, old_data, now());
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, previous_data, created_at)
        VALUES ('UPDATE', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, NEW.id, new_data, old_data, now());
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO audit.audit_events (operation, org_id, tenant_id, user_id, api_key_id, "table", entity_id, data, created_at)
        VALUES ('INSERT', scope_org_id, scope_tenant_id, actor_user_id, actor_api_key_id, TG_TABLE_NAME, NEW.id, new_data, now());
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_oidc_configurations_audit
AFTER INSERT OR UPDATE OR DELETE ON tenant_oidc_configurations
FOR EACH ROW EXECUTE FUNCTION global_audit();

CREATE TRIGGER tenant_domains_audit
AFTER INSERT OR UPDATE OR DELETE ON tenant_domains
FOR EACH ROW EXECUTE FUNCTION global_audit();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE tenant_domains;
DROP TABLE tenant_oidc_configurations;

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


-- +goose StatementEnd
