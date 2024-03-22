-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS
      transfer_mappings (
            id uuid DEFAULT uuid_generate_v4 () PRIMARY KEY,
            created_at timestamp with time zone DEFAULT now() NOT NULL,
            organization_id uuid REFERENCES organizations ON DELETE CASCADE NOT NULL,
            client_transfer_id varchar(60) NOT NULL
      );

ALTER TABLE organizations
ADD COLUMN transfer_check_scenario_id uuid REFERENCES scenarios (id) ON DELETE SET NULL;

ALTER TABLE decisions
ALTER COLUMN trigger_object
SET DATA TYPE jsonb USING trigger_object::jsonb;

CREATE INDEX IF NOT EXISTS decision_object_id_idx ON decisions (org_id, (trigger_object ->> 'object_id'));

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE transfers;

ALTER TABLE organizations
DROP COLUMN transfer_check_scenario_id;

ALTER TABLE decisions
ALTER COLUMN trigger_object
SET DATA TYPE json USING trigger_object::json;

DROP INDEX decision_object_id_idx;

-- +goose StatementEnd