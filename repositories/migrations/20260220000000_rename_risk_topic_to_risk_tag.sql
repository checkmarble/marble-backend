-- +goose NO TRANSACTION
-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS idx_entity_annotations_risk_topic_json;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_entity_annotations_risk_tag_json
ON entity_annotations (org_id, object_type, object_id, annotation_type, (payload->>'tag'))
WHERE deleted_at IS NULL AND annotation_type = 'risk_tag';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_entity_annotations_risk_tag_json;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_entity_annotations_risk_topic_json
ON entity_annotations (org_id, object_type, object_id, annotation_type, (payload->>'topic'))
WHERE deleted_at IS NULL AND annotation_type = 'risk_topic';
