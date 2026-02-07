-- +goose Up
-- Webhook endpoints configuration (replaces Convoy endpoints)
CREATE TABLE webhooks (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL DEFAULT '{}',
    http_timeout_seconds INT DEFAULT 30,
    rate_limit INT,
    rate_limit_duration_seconds INT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_webhooks_org_enabled ON webhooks(organization_id)
    WHERE deleted_at IS NULL AND enabled = true;

-- Webhook signing secrets with rotation support
CREATE TABLE webhook_secrets (
    id UUID PRIMARY KEY,
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    secret_value TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_webhook_secrets_webhook ON webhook_secrets(webhook_id)
    WHERE revoked_at IS NULL;

-- Events queued for new delivery system (separate from Convoy's webhook_events)
CREATE TABLE webhook_events_v2 (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_type VARCHAR NOT NULL,
    api_version VARCHAR NOT NULL DEFAULT 'v1',
    event_data JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_webhook_events_v2_org ON webhook_events_v2(organization_id);
CREATE INDEX idx_webhook_events_v2_created ON webhook_events_v2(created_at);

-- Per-endpoint delivery tracking
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY,
    webhook_event_id UUID NOT NULL REFERENCES webhook_events_v2(id) ON DELETE CASCADE,
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    status VARCHAR NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'success', 'failed')),
    attempts INT DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    last_error TEXT,
    last_response_status INT,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_webhook_deliveries_event ON webhook_deliveries(webhook_event_id);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_pending ON webhook_deliveries(status, next_retry_at)
    WHERE status = 'pending';

-- Unique constraint to ensure idempotent delivery creation
CREATE UNIQUE INDEX idx_webhook_deliveries_unique ON webhook_deliveries(webhook_event_id, webhook_id);

-- +goose Down
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_events_v2;
DROP TABLE IF EXISTS webhook_secrets;
DROP TABLE IF EXISTS webhooks;
