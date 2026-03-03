-- +goose Up
CREATE TABLE async_decision_executions (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL REFERENCES organizations(id),
    object_type TEXT NOT NULL,
    trigger_object JSONB NOT NULL,
    scenario_id TEXT,
    should_ingest BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'pending',
    decision_ids UUID[],
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_async_decision_executions_org_id ON async_decision_executions(org_id);
CREATE INDEX idx_async_decision_executions_status ON async_decision_executions(status);
CREATE INDEX idx_async_decision_executions_created_at ON async_decision_executions(created_at);

-- +goose Down
DROP TABLE IF EXISTS async_decision_executions;
