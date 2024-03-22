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

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE transfers;

ALTER TABLE organizations
DROP COLUMN transfer_check_scenario_id;

-- +goose StatementEnd