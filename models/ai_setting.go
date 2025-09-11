// Package models contains the models for the AI settings for different AI usecases
package models

const (
	AiCaseReviewDefaultLanguage   = "en"
	AiKYCEnrichmentDefaultEnabled = false
)

type PerplexitySearchContextSize string

const (
	PerplexitySearchContextSizeUnknown PerplexitySearchContextSize = "unknown"
	PerplexitySearchContextSizeLow     PerplexitySearchContextSize = "low"
	PerplexitySearchContextSizeMedium  PerplexitySearchContextSize = "medium"
	PerplexitySearchContextSizeHigh    PerplexitySearchContextSize = "high"
)

func PerplexitySearchContextSizeFromString(s string) PerplexitySearchContextSize {
	switch s {
	case "low":
		return PerplexitySearchContextSizeLow
	case "medium":
		return PerplexitySearchContextSizeMedium
	case "high":
		return PerplexitySearchContextSizeHigh
	}
	return PerplexitySearchContextSizeUnknown
}

type AiSettingEntity interface {
	entityAiSetting()
}

// Json tag for json serialization into JSONB column
type KYCEnrichmentSetting struct {
	Model              *string                      `json:"model"`
	DomainFilter       []string                     `json:"domain_filter"`
	SearchContextSize  *PerplexitySearchContextSize `json:"search_context_size"`
	CustomInstructions *string                      `json:"custom_instructions"`

	// Opt-in, the user should explicitly enable it
	Enabled bool `json:"enabled"`
}

func (KYCEnrichmentSetting) entityAiSetting() {}

func DefaultKYCEnrichmentSetting() KYCEnrichmentSetting {
	return KYCEnrichmentSetting{
		Enabled:      AiKYCEnrichmentDefaultEnabled,
		DomainFilter: []string{},
	}
}

// Json tag for json serialization into JSONB column
type CaseReviewSetting struct {
	Language       string  `json:"language"`
	Structure      *string `json:"structure"`
	OrgDescription *string `json:"org_description"`
}

func (CaseReviewSetting) entityAiSetting() {}

func DefaultCaseReviewSetting() CaseReviewSetting {
	return CaseReviewSetting{
		Language: AiCaseReviewDefaultLanguage,
	}
}

// AiSetting contains the settings for the AI usecases, each usecase setting is stored in a separate struct
// All fields are optional, if not set, let the usecase use a default value
type AiSetting struct {
	// Perplexity, KYC enrichment usecase
	KYCEnrichmentSetting KYCEnrichmentSetting

	// CaseReview usecase (not used yet)
	CaseReviewSetting CaseReviewSetting
}

func DefaultAiSetting() AiSetting {
	return AiSetting{
		KYCEnrichmentSetting: DefaultKYCEnrichmentSetting(),
		CaseReviewSetting:    DefaultCaseReviewSetting(),
	}
}

type UpsertAiSetting struct {
	KYCEnrichmentSetting KYCEnrichmentSetting
	CaseReviewSetting    CaseReviewSetting
}
