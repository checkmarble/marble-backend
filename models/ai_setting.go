// Package models contains the models for the AI settings for different AI usecases
package models

import (
	"time"

	"github.com/google/uuid"
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

type KYCEnrichmentSetting struct {
	Model             *string
	DomainFilter      []string
	SearchContextSize *PerplexitySearchContextSize
}

type CaseReviewSetting struct {
	Language       *string
	Structure      *string
	OrgDescription *string // Hum ... In CaseReview or put in AiSetting as a common field
}

// AiSetting contains the settings for the AI usecases, each usecase setting is stored in a separate struct
// All fields are optional, if not set, let the usecase use a default value
type AiSetting struct {
	OrgId     uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	// Perplexity, KYC enrichment usecase
	KYCEnrichmentSetting KYCEnrichmentSetting

	// CaseReview usecase (not used yet)
	CaseReviewSetting CaseReviewSetting
}

type UpsertAiSetting struct {
	KYCEnrichmentSetting KYCEnrichmentSetting
	CaseReviewSetting    CaseReviewSetting
}
