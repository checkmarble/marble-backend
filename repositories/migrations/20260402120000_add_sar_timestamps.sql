-- +goose Up
ALTER TABLE suspicious_activity_reports
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN completed_at timestamptz;

UPDATE suspicious_activity_reports SET updated_at = created_at;
UPDATE suspicious_activity_reports SET completed_at = created_at WHERE status = 'completed';

-- +goose Down
ALTER TABLE suspicious_activity_reports
    DROP COLUMN updated_at,
    DROP COLUMN completed_at;
