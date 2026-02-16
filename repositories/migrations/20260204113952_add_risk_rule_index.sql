-- +goose Up
-- +goose NO TRANSACTION

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_object_metadata_risk_topics_length
ON object_metadata ((jsonb_array_length(metadata->'topics')))
WHERE metadata_type = 'risk_topics';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_object_metadata_risk_topics_length;
