-- +goose Up
-- +goose StatementBegin
CREATE TABLE organization_entitlements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL,
    feature_id UUID NOT NULL,
    availability availability_kind NOT NULL DEFAULT 'disabled',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_org FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_feature FOREIGN KEY(feature_id) REFERENCES features(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX entitlements_unique_organization_feature ON organization_entitlements (organization_id, feature_id) WHERE deleted_at IS NULL;

CREATE TYPE availability_kind AS ENUM (
    'enabled',
    'disabled',
    'test'
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX entitlements_unique_organization_feature;
DROP TABLE organization_entitlements;
-- +goose StatementEnd
