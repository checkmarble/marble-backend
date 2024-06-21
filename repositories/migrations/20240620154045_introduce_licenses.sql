-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS
      licenses (
            id uuid DEFAULT uuid_generate_v4 (),
            key VARCHAR NOT NULL,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
            suspended_at TIMESTAMP WITH TIME ZONE,
            expiration_date TIMESTAMP WITH TIME ZONE NOT NULL,
            name VARCHAR NOT NULL,
            description VARCHAR NOT NULL,
            sso_entitlement BOOLEAN NOT NULL,
            workflows_entitlement BOOLEAN NOT NULL,
            analytics_entitlement BOOLEAN NOT NULL,
            data_enrichment BOOLEAN NOT NULL,
            user_roles BOOLEAN NOT NULL,
            PRIMARY KEY(id)
      );

CREATE UNIQUE INDEX IF NOT EXISTS idx_key ON licenses(key);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_key;

DROP TABLE licenses;

-- +goose StatementEnd