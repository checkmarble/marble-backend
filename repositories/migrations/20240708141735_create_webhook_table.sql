-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS
      webhooks (
            id uuid DEFAULT uuid_generate_v4 (),
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
            send_attempt_count INT NOT NULL DEFAULT 0,
            delivery_status VARCHAR NOT NULL,
            organization_id VARCHAR NOT NULL,
            partner_id VARCHAR,
            event_type VARCHAR NOT NULL,
            event_data json,
            PRIMARY KEY(id)
      );


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE webhooks;

-- +goose StatementEnd
