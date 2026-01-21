-- +goose Up
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_continuous_screenings_org_opensanction_entity
ON continuous_screenings (org_id, opensanction_entity_id)
WHERE opensanction_entity_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_continuous_screenings_org_opensanction_entity;
