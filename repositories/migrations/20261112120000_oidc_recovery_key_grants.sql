-- +goose Up
ALTER TABLE grants DROP CONSTRAINT grants_api_key_scope_check;
ALTER TABLE grants ADD CONSTRAINT grants_api_key_scope_check CHECK (
    principal_type <> 'api_key' OR
    (role = 'OIDC_CONFIG_EDITOR' AND tenant_id IS NOT NULL AND organization_id IS NULL) OR
    (role <> 'OIDC_CONFIG_EDITOR' AND organization_id IS NOT NULL AND tenant_id IS NULL)
);

-- +goose Down
-- Tenant-scoped API key grants must be removed before rolling back.
ALTER TABLE grants DROP CONSTRAINT grants_api_key_scope_check;
ALTER TABLE grants ADD CONSTRAINT grants_api_key_scope_check CHECK (
    principal_type <> 'api_key' OR organization_id IS NOT NULL
);
