-- +goose Up
-- +goose StatementBegin
ALTER TABLE organization_feature_access
    ADD COLUMN ai_rule_building VARCHAR NOT NULL DEFAULT 'restricted';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE organization_feature_access
    DROP COLUMN ai_rule_building;
-- +goose StatementEnd
