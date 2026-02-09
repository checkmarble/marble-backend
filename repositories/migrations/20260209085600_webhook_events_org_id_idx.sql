-- +goose Up
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY IF NOT EXISTS webhook_events_retry_by_org ON webhook_events (organization_id, created_at DESC)
where
    (
        (delivery_status)::text = ANY ((ARRAY['scheduled'::character varying, 'retry'::character varying])::text[])
    );

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS webhook_events_retry_by_org;