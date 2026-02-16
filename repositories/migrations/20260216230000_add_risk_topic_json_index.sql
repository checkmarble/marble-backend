-- +goose Up
-- +goose NO TRANSACTION

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_entity_annotations_risk_topic_json
ON entity_annotations (org_id, object_type, object_id, annotation_type, (payload->>'topic'))
WHERE deleted_at IS NULL AND annotation_type = 'risk_topic';

-- +goose Down

DROP INDEX IF EXISTS idx_entity_annotations_risk_topic_json;
