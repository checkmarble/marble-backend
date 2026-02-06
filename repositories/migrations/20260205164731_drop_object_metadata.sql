-- +goose Up
-- +goose StatementBegin
DROP TABLE object_metadata;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE TABLE object_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    object_type TEXT NOT NULL,
    object_id TEXT NOT NULL,
    metadata_type TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uniq_object_metadata
    ON object_metadata (org_id, object_type, object_id, metadata_type);

CREATE INDEX idx_object_metadata_risk_topics_gin
    ON object_metadata USING GIN ((metadata->'topics'))
    WHERE metadata_type = 'risk_topics';

CREATE INDEX idx_object_metadata_risk_topics_length
ON object_metadata ((jsonb_array_length(metadata->'topics')))
WHERE metadata_type = 'risk_topics';


CREATE TRIGGER audit
AFTER INSERT OR UPDATE OR DELETE
ON object_metadata
FOR EACH ROW EXECUTE FUNCTION global_audit();

-- +goose StatementEnd
