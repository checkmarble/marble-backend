-- +goose Up
-- +goose StatementBegin

UPDATE users SET email = lower(trim(email));

DROP INDEX IF EXISTS users_email_idx;
CREATE UNIQUE INDEX users_email_idx ON users (lower(email)) WHERE deleted_at IS NULL;

INSERT INTO grants (id, principal_type, principal_id, principal_authority, organization_id, role)
SELECT uuid_generate_v4(), 'user', u.id::text, 'marble', u.organization_id,
       CASE u.role
           WHEN 1 THEN 'VIEWER' WHEN 2 THEN 'BUILDER' WHEN 3 THEN 'PUBLISHER'
           WHEN 4 THEN 'ADMIN' WHEN 8 THEN 'ANALYST' ELSE 'NO_ROLE'
       END
FROM users u
WHERE u.organization_id IS NOT NULL
  AND u.organization_id <> '00000000-0000-0000-0000-000000000000'
  AND u.deleted_at IS NULL
  AND u.role IN (1, 2, 3, 4, 8)
ON CONFLICT DO NOTHING;

INSERT INTO grants (id, principal_type, principal_id, principal_authority, role)
SELECT uuid_generate_v4(), 'user', u.id::text, 'marble', 'MARBLE_ADMIN'
FROM users u
WHERE (u.organization_id IS NULL OR u.organization_id = '00000000-0000-0000-0000-000000000000')
  AND u.deleted_at IS NULL AND u.role = 6
ON CONFLICT DO NOTHING;

INSERT INTO grants (id, principal_type, principal_id, principal_authority, organization_id, role)
SELECT uuid_generate_v4(), 'api_key', k.id::text, 'marble', k.org_id,
       CASE k.role
           WHEN 1 THEN 'VIEWER' WHEN 2 THEN 'BUILDER' WHEN 3 THEN 'PUBLISHER'
           WHEN 4 THEN 'ADMIN' WHEN 5 THEN 'API_CLIENT' WHEN 8 THEN 'ANALYST'
           ELSE 'API_CLIENT'
       END
FROM api_keys k
WHERE k.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- TODO(MAR-2251): drop users.role, users.organization_id and api_keys.role
-- after the legacy JWT compatibility window has elapsed.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS users_email_idx;
CREATE UNIQUE INDEX users_email_idx ON users (email) WHERE deleted_at IS NULL;

-- Backfilled grants are intentionally retained; later grants may share these rows.

-- +goose StatementEnd
