-- +goose Up
-- +goose StatementBegin
CREATE TABLE ai_settings (
    org_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    kyc_enrichment_model TEXT,
    kyc_enrichment_domain_filter TEXT[],
    kyc_enrichment_search_context_size TEXT,
    case_review_language TEXT,
    case_review_structure TEXT,
    case_review_org_description TEXT,
    PRIMARY KEY (org_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE ai_settings;
-- +goose StatementEnd
