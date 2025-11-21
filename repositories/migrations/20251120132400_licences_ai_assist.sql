-- +goose Up
-- +goose StatementBegin
ALTER TABLE licenses
ADD COLUMN case_ai_assist BOOL NOT NULL DEFAULT FALSE;

ALTER TABLE organization_feature_access
ADD COLUMN case_ai_assist text NOT NULL DEFAULT 'allowed';

UPDATE organization_feature_access
SET
    case_ai_assist = 'allowed'
WHERE
    org_id IN (
        SELECT
            id
        FROM
            organizations
        WHERE
            ai_case_review_enabled = true
    );

-- +goose StatementEnd
-- +goose Down
ALTER TABLE organization_feature_access
DROP COLUMN case_ai_assist;

ALTER TABLE licenses
DROP COLUMN case_ai_assist;

-- +goose StatementBegin
-- +goose StatementEnd