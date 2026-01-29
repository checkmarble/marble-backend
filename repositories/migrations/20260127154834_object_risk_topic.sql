-- +goose Up
-- +goose StatementBegin
CREATE TABLE object_risk_topics (
    id UUID PRIMARY KEY default gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    object_type TEXT NOT NULL,
    object_id TEXT NOT NULL,
    topics TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uniq_object_risk_topic ON object_risk_topics (org_id, object_type, object_id);
CREATE INDEX idx_object_risk_topics_gin ON object_risk_topics USING GIN (topics);

CREATE TABLE object_risk_topic_events (
    id UUID PRIMARY KEY default gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    object_risk_topics_id UUID NOT NULL REFERENCES object_risk_topics(id) ON DELETE CASCADE,
    topics TEXT[] NOT NULL,
    source_type TEXT NOT NULL CONSTRAINT object_risk_topic_events_source_type_check CHECK (source_type IN ('continuous_screening_match_review', 'manual')),
    source_details JSONB,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_object_risk_topic_events_created_at ON object_risk_topic_events (object_risk_topics_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE object_risk_topic_events;
DROP TABLE object_risk_topics;
-- +goose StatementEnd
