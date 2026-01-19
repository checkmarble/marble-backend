-- +goose Up
ALTER TABLE organizations ADD COLUMN sentry_replay_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE organizations DROP COLUMN sentry_replay_enabled;
