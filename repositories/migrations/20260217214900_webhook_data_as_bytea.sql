-- +goose Up
-- +goose StatementBegin
ALTER TABLE webhook_events_v2
ALTER COLUMN event_data
TYPE BYTEA USING event_data::text::BYTEA;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE webhook_events_v2
ALTER COLUMN event_data
TYPE JSONB USING event_data::text::JSONB;

-- +goose StatementEnd