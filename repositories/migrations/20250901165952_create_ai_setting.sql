-- +goose Up
-- +goose StatementBegin
CREATE TABLE ai_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    type TEXT NOT NULL,
    value JSONB NOT NULL
);
-- Need to be updated if we introduce new foreign key constraints
CREATE UNIQUE INDEX idx_ai_settings_org_id_type ON ai_settings (org_id, type);
CREATE INDEX idx_ai_settings_org_id ON ai_settings (org_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE ai_settings;
-- +goose StatementEnd
