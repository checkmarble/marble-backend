-- +goose Up
-- +goose StatementBegin
CREATE TABLE organization_feature_access (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL,
    test_run VARCHAR NOT NULL DEFAULT 'allow',
    workflows VARCHAR NOT NULL DEFAULT 'allow',
    webhooks VARCHAR NOT NULL DEFAULT 'allow',
    rule_snoozed VARCHAR NOT NULL DEFAULT 'allow',
    roles VARCHAR NOT NULL DEFAULT 'allow',
    analytics VARCHAR NOT NULL DEFAULT 'allow',
    sanctions VARCHAR NOT NULL DEFAULT 'allow',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX unique_organization_feature_access ON organization_feature_access (org_id) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX unique_organization_feature_access;
DROP TABLE organization_feature_access;
-- +goose StatementEnd
