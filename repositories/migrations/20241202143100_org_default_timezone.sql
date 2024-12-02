-- +goose Up
-- +goose StatementBegin
ALTER TABLE organizations
ADD COLUMN default_scenario_timezone text;

ALTER TABLE organizations
DROP COLUMN export_scheduled_execution_s3;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE organizations
DROP COLUMN default_scenario_timezone;

ALTER TABLE organizations
ADD COLUMN export_scheduled_execution_s3 VARCHAR() DEFAULT '';

-- +goose StatementEnd