-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS
      partners (
            id uuid DEFAULT uuid_generate_v4 () PRIMARY KEY,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
            name varchar(255) NOT NULL
      );

ALTER TABLE api_keys
ADD COLUMN IF NOT EXISTS partner_id uuid REFERENCES partners (id) ON DELETE SET NULL;

DELETE FROM transfer_mappings
WHERE
      TRUE;

ALTER TABLE transfer_mappings
ADD COLUMN IF NOT EXISTS partner_id uuid NOT NULL REFERENCES partners (id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS transfer_mappings_client_transfer_id_idx ON transfer_mappings (organization_id, partner_id, client_transfer_id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE api_keys
DROP COLUMN partner_id;

DROP INDEX IF EXISTS transfer_mappings_client_transfer_id_idx;

ALTER TABLE transfer_mappings
DROP COLUMN partner_id;

DROP TABLE partners;

-- +goose StatementEnd