-- +goose Up
-- +goose StatementBegin
ALTER TABLE organization_feature_access
ADD COLUMN IF NOT EXISTS continuous_screening TEXT NOT NULL DEFAULT 'allowed';

ALTER TABLE licenses
ADD COLUMN IF NOT EXISTS continuous_screening BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE organization_feature_access
DROP COLUMN continuous_screening;

ALTER TABLE licenses
DROP COLUMN continuous_screening;
-- +goose StatementEnd
