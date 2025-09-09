package dbmodels

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBAiSetting struct {
	Id        uuid.UUID      `db:"id"`
	OrgId     string         `db:"org_id"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
	Type      string         `db:"type"`
	Value     map[string]any `db:"value"`
}

const TABLE_AI_SETTING = "ai_settings"

const (
	AI_SETTING_TYPE_KYC_ENRICHMENT = "kyc_enrichment"
	AI_SETTING_TYPE_CASE_REVIEW    = "case_review"
)

var AiSettingColumns = utils.ColumnList[DBAiSetting]()

// AdaptAiSetting adapts multiple DB records into a single AiSetting model
func AdaptAiSetting(settings []DBAiSetting, orgId string) (models.AiSetting, error) {
	if len(settings) == 0 {
		return models.AiSetting{}, fmt.Errorf("no settings provided")
	}

	aiSetting := models.AiSetting{
		OrgId: orgId,
	}

	for _, setting := range settings {
		switch setting.Type {
		case AI_SETTING_TYPE_KYC_ENRICHMENT:
			kycSetting, err := adaptKYCEnrichmentFromJSONB(setting.Value)
			if err != nil {
				return models.AiSetting{}, fmt.Errorf("failed to adapt KYC enrichment setting: %w", err)
			}
			aiSetting.KYCEnrichmentSetting = &kycSetting

		case AI_SETTING_TYPE_CASE_REVIEW:
			caseReviewSetting, err := adaptCaseReviewFromJSONB(setting.Value)
			if err != nil {
				return models.AiSetting{}, fmt.Errorf("failed to adapt case review setting: %w", err)
			}
			aiSetting.CaseReviewSetting = &caseReviewSetting
		}
	}

	return aiSetting, nil
}

func adaptKYCEnrichmentFromJSONB(value map[string]any) (models.KYCEnrichmentSetting, error) {
	setting := models.KYCEnrichmentSetting{}

	if model, ok := value["model"].(string); ok && model != "" {
		setting.Model = &model
	}

	if domainFilterRaw, ok := value["domain_filter"].([]any); ok {
		setting.DomainFilter = make([]string, len(domainFilterRaw))
		for i, v := range domainFilterRaw {
			if str, ok := v.(string); ok {
				setting.DomainFilter[i] = str
			} else {
				return models.KYCEnrichmentSetting{}, fmt.Errorf(
					"domain filter contains non-string value",
				)
			}
		}
	}

	if searchContextStr, ok := value["search_context_size"].(string); ok && searchContextStr != "" {
		searchContext := models.PerplexitySearchContextSizeFromString(searchContextStr)
		setting.SearchContextSize = &searchContext
	}

	if enabled, ok := value["enabled"].(bool); ok {
		setting.Enabled = &enabled
	}

	return setting, nil
}

func adaptCaseReviewFromJSONB(value map[string]any) (models.CaseReviewSetting, error) {
	setting := models.CaseReviewSetting{}

	if language, ok := value["language"].(string); ok && language != "" {
		setting.Language = &language
	}

	if structure, ok := value["structure"].(string); ok && structure != "" {
		setting.Structure = &structure
	}

	if orgDesc, ok := value["org_description"].(string); ok && orgDesc != "" {
		setting.OrgDescription = &orgDesc
	}

	return setting, nil
}
