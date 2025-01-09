-- +goose Up
-- +goose StatementBegin
CREATE TABLE
    organization_feature_access (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
        org_id UUID NOT NULL,
        test_run VARCHAR NOT NULL DEFAULT 'allowed',
        sanctions VARCHAR NOT NULL DEFAULT 'allowed',
        created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
        CONSTRAINT fk_org FOREIGN KEY (org_id) REFERENCES organizations (id) ON DELETE CASCADE
    );

INSERT INTO
    organization_feature_access (org_id)
SELECT
    id
FROM
    organizations;

CREATE UNIQUE INDEX unique_organization_feature_access ON organization_feature_access (org_id);

ALTER TABLE licenses
ADD COLUMN sanctions BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX unique_organization_feature_access;

DROP TABLE organization_feature_access;

ALTER TABLE licenses
DROP COLUMN sanctions;

-- +goose StatementEnd