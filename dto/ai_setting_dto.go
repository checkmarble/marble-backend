package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
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
	Id        uuid.UUID `json:"id" binding:"required"`
	CreatedAt time.Time `json:"created_at" binding:"required"`
	UpdatedAt time.Time `json:"updated_at" binding:"required"`

	// Perplexity, KYC enrichment usecase
	KYCEnrichmentSetting KYCEnrichmentSettingDto `json:"kyc_enrichment_setting" binding:"required"`

	// CaseReview usecase (not used yet)
	CaseReviewSetting CaseReviewSettingDto `json:"case_review_setting" binding:"required"`
}

func AdaptAiSettingDto(setting models.AiSetting) AiSettingDto {
	return AiSettingDto{
		Id:                   setting.Id,
		CreatedAt:            setting.CreatedAt,
		UpdatedAt:            setting.UpdatedAt,
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

func (dto UpsertAiSettingDto) Validate() error {
	if err := dto.KYCEnrichmentSetting.Validate(); err != nil {
		return err
	}
	if err := dto.CaseReviewSetting.Validate(); err != nil {
		return err
	}
	return nil
}

func AdaptUpsertAiSetting(setting UpsertAiSettingDto) models.UpsertAiSetting {
	return models.UpsertAiSetting{
		KYCEnrichmentSetting: AdaptKYCEnrichmentSetting(setting.KYCEnrichmentSetting),
		CaseReviewSetting:    AdaptCaseReviewSetting(setting.CaseReviewSetting),
	}
}
