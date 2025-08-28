package dbmodels

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBAiSetting struct {
	OrgId uuid.UUID `db:"org_id"`

	KYCEnrichmentModel             *string  `db:"kyc_enrichment_model"`
	KYCEnrichmentDomainFilter      []string `db:"kyc_enrichment_domain_filter"`
	KYCEnrichmentSearchContextSize *string  `db:"kyc_enrichment_search_context_size"`

	CaseReviewLanguage       *string `db:"case_review_language"`
	CaseReviewStructure      *string `db:"case_review_structure"`
	CaseReviewOrgDescription *string `db:"case_review_org_description"`
}

const TABLE_AI_SETTING = "ai_settings"

var AiSettingColumns = utils.ColumnList[DBAiSetting]()

func AdaptAiSetting(db DBAiSetting) (models.AiSetting, error) {
	var kycEnrichmentSearchContextSize *models.PerplexitySearchContextSize
	if db.KYCEnrichmentSearchContextSize != nil {
		contextSize := models.PerplexitySearchContextSizeFromString(*db.KYCEnrichmentSearchContextSize)
		if contextSize == models.PerplexitySearchContextSizeUnknown {
			return models.AiSetting{}, errors.New("invalid kyc enrichment search context size from database")
		}
		kycEnrichmentSearchContextSize = &contextSize
	}

	return models.AiSetting{
		OrgId: db.OrgId,
		KYCEnrichmentSetting: models.KYCEnrichmentSetting{
			Model:             db.KYCEnrichmentModel,
			DomainFilter:      db.KYCEnrichmentDomainFilter,
			SearchContextSize: kycEnrichmentSearchContextSize,
		},
		CaseReviewSetting: models.CaseReviewSetting{
			Language:       db.CaseReviewLanguage,
			Structure:      db.CaseReviewStructure,
			OrgDescription: db.CaseReviewOrgDescription,
		},
	}, nil
}
