package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"golang.org/x/text/language"
)

type KYCEnrichmentSettingDto struct {
	Model              *string                             `json:"model"`
	DomainFilter       []string                            `json:"domain_filter"`
	SearchContextSize  *models.PerplexitySearchContextSize `json:"search_context_size"`
	CustomInstructions *string                             `json:"custom_instructions"`
	Enabled            bool                                `json:"enabled"`
}

func (dto KYCEnrichmentSettingDto) Validate() error {
	return nil
}

func AdaptKYCEnrichmentSettingDto(setting models.KYCEnrichmentSetting) KYCEnrichmentSettingDto {
	return KYCEnrichmentSettingDto{
		Model:              setting.Model,
		DomainFilter:       setting.DomainFilter,
		SearchContextSize:  setting.SearchContextSize,
		CustomInstructions: setting.CustomInstructions,
		Enabled:            setting.Enabled,
	}
}

func AdaptKYCEnrichmentSetting(setting KYCEnrichmentSettingDto) models.KYCEnrichmentSetting {
	return models.KYCEnrichmentSetting{
		Model:              setting.Model,
		DomainFilter:       setting.DomainFilter,
		SearchContextSize:  setting.SearchContextSize,
		CustomInstructions: setting.CustomInstructions,
		Enabled:            setting.Enabled,
	}
}

type CaseReviewSettingDto struct {
	Language       string  `json:"language" binding:"required"`
	Structure      *string `json:"structure"`
	OrgDescription *string `json:"org_description"`
}

func (dto CaseReviewSettingDto) Validate() error {
	_, err := language.Parse(dto.Language)
	if err != nil {
		return err
	}

	return nil
}

func AdaptCaseReviewSettingDto(setting models.CaseReviewSetting) CaseReviewSettingDto {
	return CaseReviewSettingDto{
		Language:       setting.Language,
		Structure:      setting.Structure,
		OrgDescription: setting.OrgDescription,
	}
}

func AdaptCaseReviewSetting(setting CaseReviewSettingDto) models.CaseReviewSetting {
	return models.CaseReviewSetting{
		Language:       setting.Language,
		Structure:      setting.Structure,
		OrgDescription: setting.OrgDescription,
	}
}

type AiSettingDto struct {
	// Perplexity, KYC enrichment usecase
	KYCEnrichmentSetting KYCEnrichmentSettingDto `json:"kyc_enrichment_setting" binding:"required"`

	// CaseReview usecase (not used yet)
	CaseReviewSetting CaseReviewSettingDto `json:"case_review_setting" binding:"required"`
}

func AdaptAiSettingDto(setting models.AiSetting) AiSettingDto {
	return AiSettingDto{
		KYCEnrichmentSetting: AdaptKYCEnrichmentSettingDto(setting.KYCEnrichmentSetting),
		CaseReviewSetting:    AdaptCaseReviewSettingDto(setting.CaseReviewSetting),
	}
}

// PATCH semantics: each field is optional, only provided fields are updated in DB, others remain unchanged
type PutAiSettingDto struct {
	KYCEnrichmentSetting KYCEnrichmentSettingDto `json:"kyc_enrichment_setting" binding:"required"`
	CaseReviewSetting    CaseReviewSettingDto    `json:"case_review_setting" binding:"required"`
}

func (dto PutAiSettingDto) Validate() error {
	if err := dto.KYCEnrichmentSetting.Validate(); err != nil {
		return err
	}
	if err := dto.CaseReviewSetting.Validate(); err != nil {
		return err
	}
	return nil
}

func AdaptPutAiSetting(setting PutAiSettingDto) models.UpsertAiSetting {
	result := models.UpsertAiSetting{}

	result.KYCEnrichmentSetting = AdaptKYCEnrichmentSetting(setting.KYCEnrichmentSetting)

	result.CaseReviewSetting = AdaptCaseReviewSetting(setting.CaseReviewSetting)

	return result
}
