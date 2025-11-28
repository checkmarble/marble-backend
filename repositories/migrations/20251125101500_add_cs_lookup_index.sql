-- +goose NO TRANSACTION
-- +goose Up

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_continuous_screenings_org_object_status_created ON continuous_screenings (org_id, object_type, object_id, status, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_continuous_screenings_org_object_status_created;