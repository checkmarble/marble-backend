-- +goose Up
ALTER TABLE users ADD COLUMN tenant_id uuid REFERENCES tenants(id);
UPDATE users u SET tenant_id = o.tenant_id
FROM organizations o WHERE o.id = u.organization_id AND u.role NOT IN (6, 10);

-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE deleted_at IS NULL AND role NOT IN (6, 10) AND tenant_id IS NULL) THEN
        RAISE EXCEPTION 'Active non-platform users without a tenant: repair their organization before migrating';
    END IF;
END $$;
-- +goose StatementEnd

CREATE INDEX users_tenant_id_idx ON users(tenant_id);
ALTER TABLE users DROP COLUMN organization_id;

-- +goose Down
-- A tenant identity can have zero or multiple organization grants. No home organization can be reconstructed.
-- +goose StatementBegin
DO $$ BEGIN
    RAISE EXCEPTION 'Irreversible identity migration: restore the pre-migration backup during maintenance';
END $$;
-- +goose StatementEnd
