package dbmodels

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

// DBAiSetting is the database model for the AI setting
// For a given organization, there can be multiple settings for different AI usecases
// Value of each usecase setting is stored in the Value field as a JSONB column, need to parse it to the correct type
type DBAiSetting struct {
	Id        uuid.UUID      `db:"id"`
	OrgId     uuid.UUID      `db:"org_id"`
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
func AdaptAiSetting(settings []DBAiSetting) (models.AiSetting, error) {
	if len(settings) == 0 {
		return models.DefaultAiSetting(), nil
	}

	aiSetting := models.DefaultAiSetting()

	for _, setting := range settings {
		switch setting.Type {
		case AI_SETTING_TYPE_KYC_ENRICHMENT:
			kycSetting, err := adaptKYCEnrichmentFromJSONB(setting.Value)
			if err != nil {
				return models.DefaultAiSetting(), fmt.Errorf(
					"failed to adapt KYC enrichment setting: %w", err)
			}
			aiSetting.KYCEnrichmentSetting = kycSetting

		case AI_SETTING_TYPE_CASE_REVIEW:
			caseReviewSetting, err := adaptCaseReviewFromJSONB(setting.Value)
			if err != nil {
				return models.DefaultAiSetting(), fmt.Errorf(
					"failed to adapt case review setting: %w", err)
			}
			aiSetting.CaseReviewSetting = caseReviewSetting
		}
	}

	return aiSetting, nil
}

func adaptKYCEnrichmentFromJSONB(value map[string]any) (models.KYCEnrichmentSetting, error) {
	setting := models.DefaultKYCEnrichmentSetting()

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

	if customInstructions, ok := value["custom_instructions"].(string); ok && customInstructions != "" {
		setting.CustomInstructions = &customInstructions
	}

	if enabled, ok := value["enabled"].(bool); ok {
		setting.Enabled = enabled
	}

	return setting, nil
}

func adaptCaseReviewFromJSONB(value map[string]any) (models.CaseReviewSetting, error) {
	setting := models.DefaultCaseReviewSetting()

	if language, ok := value["language"].(string); ok && language != "" {
		setting.Language = language
	}

	if structure, ok := value["structure"].(string); ok && structure != "" {
		setting.Structure = &structure
	}

	if orgDesc, ok := value["org_description"].(string); ok && orgDesc != "" {
		setting.OrgDescription = &orgDesc
	}

	return setting, nil
}
