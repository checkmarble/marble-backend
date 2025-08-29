package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type KYCEnrichmentSettingDto struct {
	Model             *string                             `json:"model"`
	DomainFilter      []string                            `json:"domain_filter"`
	SearchContextSize *models.PerplexitySearchContextSize `json:"search_context_size"`
}

func AdaptKYCEnrichmentSettingDto(setting models.KYCEnrichmentSetting) KYCEnrichmentSettingDto {
	return KYCEnrichmentSettingDto{
		Model:             setting.Model,
		DomainFilter:      setting.DomainFilter,
		SearchContextSize: setting.SearchContextSize,
	}
}

func AdaptKYCEnrichmentSetting(setting KYCEnrichmentSettingDto) models.KYCEnrichmentSetting {
	return models.KYCEnrichmentSetting{
		Model:             setting.Model,
		DomainFilter:      setting.DomainFilter,
		SearchContextSize: setting.SearchContextSize,
	}
}

type CaseReviewSettingDto struct {
	Language       *string `json:"language"`
	Structure      *string `json:"structure"`
	OrgDescription *string `json:"org_description"`
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
	OrgId uuid.UUID `json:"org_id" binding:"required"`

	// Perplexity, KYC enrichment usecase
	KYCEnrichmentSetting KYCEnrichmentSettingDto `json:"kyc_enrichment_setting" binding:"required"`

	// CaseReview usecase (not used yet)
	CaseReviewSetting CaseReviewSettingDto `json:"case_review_setting" binding:"required"`
}

func AdaptAiSettingDto(setting models.AiSetting) AiSettingDto {
	return AiSettingDto{
		OrgId:                setting.OrgId,
		KYCEnrichmentSetting: AdaptKYCEnrichmentSettingDto(setting.KYCEnrichmentSetting),
		CaseReviewSetting:    AdaptCaseReviewSettingDto(setting.CaseReviewSetting),
	}
}

type UpsertAiSettingDto struct {
	// Perplexity, KYC enrichment usecase
	KYCEnrichmentSetting KYCEnrichmentSettingDto `json:"kyc_enrichment_setting" binding:"required"`

	// CaseReview usecase (not used yet)
	CaseReviewSetting CaseReviewSettingDto `json:"case_review_setting" binding:"required"`
}

func AdaptUpsertAiSetting(setting UpsertAiSettingDto) models.UpsertAiSetting {
	return models.UpsertAiSetting{
		KYCEnrichmentSetting: AdaptKYCEnrichmentSetting(setting.KYCEnrichmentSetting),
		CaseReviewSetting:    AdaptCaseReviewSetting(setting.CaseReviewSetting),
	}
}
