package dbmodels

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBAiSetting struct {
	Id        uuid.UUID              `db:"id"`
	OrgId     string                 `db:"org_id"`
	CreatedAt time.Time              `db:"created_at"`
	UpdatedAt time.Time              `db:"updated_at"`
	Type      string                 `db:"type"`
	Value     map[string]interface{} `db:"value"`
}

const TABLE_AI_SETTING = "ai_settings"

const (
	AI_SETTING_TYPE_KYC_ENRICHMENT = "kyc_enrichment"
	AI_SETTING_TYPE_CASE_REVIEW    = "case_review"
)

var (
	AiSettingColumns       = utils.ColumnList[DBAiSetting]()
	AiSettingColumnsInsert = []string{
		"id",
		"org_id",
		"type",
		"value",
	}
)

// AdaptAiSetting adapts multiple DB records into a single AiSetting model
func AdaptAiSetting(settings []DBAiSetting, orgId string) (models.AiSetting, error) {
	if len(settings) == 0 {
		return models.AiSetting{}, fmt.Errorf("no settings provided")
	}

	aiSetting := models.AiSetting{
		Id:    settings[0].Id, // All settings share the same ID
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

func adaptKYCEnrichmentFromJSONB(value map[string]interface{}) (models.KYCEnrichmentSetting, error) {
	setting := models.KYCEnrichmentSetting{}

	if model, ok := value["model"].(string); ok && model != "" {
		setting.Model = &model
	}

	if domainFilterRaw, ok := value["domain_filter"].([]interface{}); ok {
		setting.DomainFilter = make([]string, len(domainFilterRaw))
		for i, v := range domainFilterRaw {
			if str, ok := v.(string); ok {
				setting.DomainFilter[i] = str
			}
		}
	}

	if searchContextStr, ok := value["search_context_size"].(string); ok && searchContextStr != "" {
		searchContext := models.PerplexitySearchContextSizeFromString(searchContextStr)
		setting.SearchContextSize = &searchContext
	}

	return setting, nil
}

func adaptCaseReviewFromJSONB(value map[string]interface{}) (models.CaseReviewSetting, error) {
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

// Helper functions to convert models to JSONB for storage
func KYCEnrichmentToJSONB(setting models.KYCEnrichmentSetting) map[string]interface{} {
	value := make(map[string]interface{})

	if setting.Model != nil {
		value["model"] = *setting.Model
	}
	if len(setting.DomainFilter) > 0 {
		value["domain_filter"] = setting.DomainFilter
	}
	if setting.SearchContextSize != nil {
		value["search_context_size"] = string(*setting.SearchContextSize)
	}

	return value
}

func CaseReviewToJSONB(setting models.CaseReviewSetting) map[string]interface{} {
	value := make(map[string]interface{})

	if setting.Language != nil {
		value["language"] = *setting.Language
	}
	if setting.Structure != nil {
		value["structure"] = *setting.Structure
	}
	if setting.OrgDescription != nil {
		value["org_description"] = *setting.OrgDescription
	}

	return value
}
