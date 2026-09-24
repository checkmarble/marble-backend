-- +goose Up
ALTER TABLE organizations
    DROP CONSTRAINT organizations_environment_check,
    ADD CONSTRAINT organizations_environment_check
        CHECK (environment IN ('production', 'demo', 'staging'));

-- +goose Down
ALTER TABLE organizations
    DROP CONSTRAINT organizations_environment_check,
    ADD CONSTRAINT organizations_environment_check
        CHECK (environment IN ('production', 'demo'));
