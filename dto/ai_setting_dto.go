package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"golang.org/x/text/language"
)

type KYCEnrichmentSettingDto struct {
	Model             *string                             `json:"model"`
	DomainFilter      []string                            `json:"domain_filter"`
	SearchContextSize *models.PerplexitySearchContextSize `json:"search_context_size"`
}

func (dto KYCEnrichmentSettingDto) Validate() error {
	return nil
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

func (dto CaseReviewSettingDto) Validate() error {
	if dto.Language != nil {
		_, err := language.Parse(*dto.Language)
		if err != nil {
			return err
		}
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
	OrgId string `json:"org_id" binding:"required"`

	// Perplexity, KYC enrichment usecase
	KYCEnrichmentSetting *KYCEnrichmentSettingDto `json:"kyc_enrichment_setting"`

	// CaseReview usecase (not used yet)
	CaseReviewSetting *CaseReviewSettingDto `json:"case_review_setting"`
}

func AdaptAiSettingDto(setting models.AiSetting) AiSettingDto {
	var kycEnrichmentSetting *KYCEnrichmentSettingDto
	if setting.KYCEnrichmentSetting != nil {
		kycEnrichmentSetting = utils.Ptr(AdaptKYCEnrichmentSettingDto(*setting.KYCEnrichmentSetting))
	}
	var caseReviewSetting *CaseReviewSettingDto
	if setting.CaseReviewSetting != nil {
		caseReviewSetting = utils.Ptr(AdaptCaseReviewSettingDto(*setting.CaseReviewSetting))
	}
	return AiSettingDto{
		OrgId:                setting.OrgId,
		KYCEnrichmentSetting: kycEnrichmentSetting,
		CaseReviewSetting:    caseReviewSetting,
	}
}

// PATCH semantics: each field is optional, only provided fields are updated in DB, others remain unchanged
type PatchAiSettingDto struct {
	KYCEnrichmentSetting *KYCEnrichmentSettingDto `json:"kyc_enrichment_setting,omitempty"`
	CaseReviewSetting    *CaseReviewSettingDto    `json:"case_review_setting,omitempty"`
}

func (dto PatchAiSettingDto) Validate() error {
	if dto.KYCEnrichmentSetting != nil {
		if err := dto.KYCEnrichmentSetting.Validate(); err != nil {
			return err
		}
	}
	if dto.CaseReviewSetting != nil {
		if err := dto.CaseReviewSetting.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func AdaptPatchAiSetting(setting PatchAiSettingDto) models.UpsertAiSetting {
	result := models.UpsertAiSetting{}

	if setting.KYCEnrichmentSetting != nil {
		result.KYCEnrichmentSetting = utils.Ptr(
			AdaptKYCEnrichmentSetting(*setting.KYCEnrichmentSetting),
		)
	}

	if setting.CaseReviewSetting != nil {
		result.CaseReviewSetting = utils.Ptr(AdaptCaseReviewSetting(*setting.CaseReviewSetting))
	}

	return result
}
