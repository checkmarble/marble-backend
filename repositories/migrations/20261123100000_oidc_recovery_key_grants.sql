-- +goose Up
ALTER TABLE grants DROP CONSTRAINT grants_api_key_scope_check;
ALTER TABLE grants ADD CONSTRAINT grants_api_key_scope_check CHECK (
    principal_type <> 'api_key' OR
    (role = 'OIDC_CONFIG_EDITOR' AND tenant_id IS NOT NULL AND organization_id IS NULL) OR
    (role <> 'OIDC_CONFIG_EDITOR' AND organization_id IS NOT NULL AND tenant_id IS NULL)
);

ALTER TABLE tenant_domains ALTER COLUMN verified_at DROP NOT NULL;
CREATE UNIQUE INDEX tenant_domains_tenant_domain_unique_idx ON tenant_domains (tenant_id, domain);

-- +goose Down
DROP INDEX tenant_domains_tenant_domain_unique_idx;
UPDATE tenant_domains SET verified_at = now() WHERE verified_at IS NULL;
ALTER TABLE tenant_domains ALTER COLUMN verified_at SET NOT NULL;

-- Tenant-scoped API key grants must be removed before rolling back.
ALTER TABLE grants DROP CONSTRAINT grants_api_key_scope_check;
ALTER TABLE grants ADD CONSTRAINT grants_api_key_scope_check CHECK (
    principal_type <> 'api_key' OR organization_id IS NOT NULL
);
