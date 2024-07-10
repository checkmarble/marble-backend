-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS
      webhook_events (
            id uuid DEFAULT uuid_generate_v4 (),
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
            send_attempt_count INT NOT NULL DEFAULT 0,
            delivery_status VARCHAR NOT NULL,
            organization_id uuid NOT NULL,
            partner_id uuid,
            event_type VARCHAR NOT NULL,
            event_data json,
            PRIMARY KEY(id),
            CONSTRAINT fk_webhooks_org FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
            CONSTRAINT fk_webhooks_partner FOREIGN KEY(partner_id) REFERENCES partners(id) ON DELETE CASCADE
      );

CREATE INDEX webhooks_delivery_status_idx ON webhook_events(delivery_status) WHERE delivery_status IN ('scheduled', 'retry');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE webhook_events;

-- +goose StatementEnd
