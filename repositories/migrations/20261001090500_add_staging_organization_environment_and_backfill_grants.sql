-- +goose Up
ALTER TABLE organizations
    DROP CONSTRAINT organizations_environment_check,
    ADD CONSTRAINT organizations_environment_check
        CHECK (environment IN ('production', 'demo', 'staging'));

INSERT INTO grants (id, principal_type, principal_id, principal_authority, tenant_id, organization_id, role)
SELECT
    uuid_generate_v4(),
    'user',
    u.id::text,
    'marble',
    CASE WHEN u.role = 11 THEN o.tenant_id END,
    CASE WHEN u.role IN (1, 2, 3, 4, 9) THEN u.organization_id END,
    CASE u.role
        WHEN 1 THEN 'VIEWER'
        WHEN 2 THEN 'BUILDER'
        WHEN 3 THEN 'PUBLISHER'
        WHEN 4 THEN 'ADMIN'
        WHEN 6 THEN 'MARBLE_ADMIN'
        WHEN 9 THEN 'ANALYST'
        WHEN 11 THEN 'TENANT_ADMIN'
    END
FROM users u
LEFT JOIN organizations o ON o.id = u.organization_id AND o.deleted_at IS NULL
WHERE u.deleted_at IS NULL
  AND (
      u.role = 6
      OR (u.role IN (1, 2, 3, 4, 9, 11) AND o.id IS NOT NULL)
  )
ON CONFLICT DO NOTHING;

INSERT INTO grants (id, principal_type, principal_id, principal_authority, organization_id, role)
SELECT
    uuid_generate_v4(),
    'api_key',
    k.id::text,
    'marble',
    k.org_id,
    CASE k.role
        WHEN 1 THEN 'VIEWER'
        WHEN 2 THEN 'BUILDER'
        WHEN 3 THEN 'PUBLISHER'
        WHEN 4 THEN 'ADMIN'
        WHEN 5 THEN 'API_CLIENT'
        WHEN 9 THEN 'ANALYST'
    END
FROM api_keys k
WHERE k.deleted_at IS NULL
  AND k.role IN (1, 2, 3, 4, 5, 9)
ON CONFLICT DO NOTHING;

-- +goose Down
ALTER TABLE organizations
    DROP CONSTRAINT organizations_environment_check,
    ADD CONSTRAINT organizations_environment_check
        CHECK (environment IN ('production', 'demo'));
