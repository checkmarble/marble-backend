-- +goose Up

alter table webhook_events rename to webhook_events_deprec;

CREATE VIEW webhook_events AS (
    SELECT
        id,
        organization_id,
        event_type,
        api_version,
        event_data,
        created_at
    FROM webhook_events_v2
);

-- +goose Down

DROP VIEW webhook_events;

alter table webhook_events_deprec rename to webhook_events;
