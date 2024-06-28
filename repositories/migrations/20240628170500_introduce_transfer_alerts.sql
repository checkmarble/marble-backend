-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS
      transfer_alerts (
            id uuid DEFAULT uuid_generate_v4 (),
            transfer_id uuid NOT NULL REFERENCES transfer_mappings (id),
            organization_id uuid NOT NULL REFERENCES organizations (id),
            sender_partner_id uuid NOT NULL REFERENCES partners (id),
            beneficiary_partner_id uuid NOT NULL REFERENCES partners (id),
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
            status VARCHAR(255) NOT NULL DEFAULT 'pending',
            message TEXT NOT NULL,
            transfer_end_to_end_id VARCHAR(255) NOT NULL,
            beneficiary_iban VARCHAR(255) NOT NULL,
            sender_iban VARCHAR(255) NOT NULL,
            PRIMARY KEY (id)
      );

CREATE UNIQUE INDEX transfer_alerts_unique_transfer_id ON transfer_alerts (transfer_id);

CREATE INDEX transfer_alerts_sender_idx ON transfer_alerts (organization_id, sender_partner_id, created_at DESC);

CREATE INDEX transfer_alerts_beneficiary_idx ON transfer_alerts (organization_id, beneficiary_partner_id, created_at DESC);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX transfer_alerts_unique_transfer_id;

DROP INDEX transfer_alerts_sender_idx;

DROP INDEX transfer_alerts_beneficiary_idx;

DROP TABLE transfer_alerts;

-- +goose StatementEnd