-- +goose Up
-- +goose StatementBegin

-- Add updated_at column for tracking when annotations are modified
ALTER TABLE entity_annotations ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE;

-- Update existing rows to have created_at as their updated_at
UPDATE entity_annotations SET updated_at = created_at WHERE updated_at IS NULL;

-- Make updated_at NOT NULL with default
ALTER TABLE entity_annotations
    ALTER COLUMN updated_at SET NOT NULL,
    ALTER COLUMN updated_at SET DEFAULT NOW();

-- GIN index for ?| operator on topics array (for MonitoringListCheck queries)
CREATE INDEX idx_entity_annotations_risk_topics_gin
    ON entity_annotations USING GIN ((payload->'topics'))
    WHERE annotation_type = 'risk_topic' AND deleted_at IS NULL;

-- Index for array length queries (check if object has any topics)
CREATE INDEX idx_entity_annotations_risk_topics_length
    ON entity_annotations ((jsonb_array_length(payload->'topics')))
    WHERE annotation_type = 'risk_topic' AND deleted_at IS NULL;

-- Audit trigger for history tracking
CREATE TRIGGER entity_annotations_audit
    AFTER INSERT OR UPDATE OR DELETE ON entity_annotations
    FOR EACH ROW EXECUTE FUNCTION global_audit();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER entity_annotations_audit ON entity_annotations;
DROP INDEX idx_entity_annotations_risk_topics_length;
DROP INDEX idx_entity_annotations_risk_topics_gin;
ALTER TABLE entity_annotations DROP COLUMN updated_at;

-- +goose StatementEnd
