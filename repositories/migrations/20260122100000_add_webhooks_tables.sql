-- +goose Up
-- +goose StatementBegin

-- Webhook endpoint configuration (replaces Convoy endpoints + subscriptions)
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    partner_id UUID REFERENCES partners(id) ON DELETE CASCADE,
    name VARCHAR,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL DEFAULT '{}',
    http_timeout_seconds INT DEFAULT 30,
    rate_limit INT,
    rate_limit_duration_seconds INT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Signing secrets with rotation support
CREATE TABLE webhook_secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    secret_value TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE
);

-- Per-endpoint delivery tracking (enables fan-out with independent retry)
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_event_id UUID NOT NULL REFERENCES webhook_events(id) ON DELETE CASCADE,
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    status VARCHAR NOT NULL DEFAULT 'pending',
    attempts INT DEFAULT 0,
    next_retry_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    last_response_status INT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

-- Indexes for webhooks
CREATE INDEX idx_webhooks_org_enabled ON webhooks(organization_id)
    WHERE deleted_at IS NULL AND enabled = true;
CREATE INDEX idx_webhooks_org_partner ON webhooks(organization_id, partner_id)
    WHERE deleted_at IS NULL;

-- Indexes for webhook_secrets
CREATE INDEX idx_webhook_secrets_webhook ON webhook_secrets(webhook_id)
    WHERE revoked_at IS NULL;

-- Indexes for webhook_deliveries
CREATE INDEX idx_webhook_deliveries_pending ON webhook_deliveries(next_retry_at)
    WHERE status = 'pending';
CREATE INDEX idx_webhook_deliveries_event ON webhook_deliveries(webhook_event_id);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);

-- Autovacuum tuning for high-volume table
ALTER TABLE webhook_deliveries SET (
    autovacuum_vacuum_insert_scale_factor = 0.02,
    autovacuum_analyze_scale_factor = 0.01
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_secrets;
DROP TABLE IF EXISTS webhooks;

-- +goose StatementEnd
