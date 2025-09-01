-- +goose Up
-- +goose StatementBegin
CREATE TABLE ai_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    kyc_enrichment_model TEXT,
    kyc_enrichment_domain_filter TEXT[],
    kyc_enrichment_search_context_size TEXT,
    case_review_language TEXT,
    case_review_structure TEXT,
    case_review_org_description TEXT
);

ALTER TABLE organizations ADD COLUMN ai_setting_id UUID REFERENCES ai_settings(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE organizations DROP COLUMN ai_setting_id;
DROP TABLE ai_settings;
-- +goose StatementEnd
