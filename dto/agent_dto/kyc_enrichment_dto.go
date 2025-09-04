package agent_dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type CitationDto struct {
	Title  string    `json:"title"`
	Domain string    `json:"domain"`
	Url    string    `json:"url"`
	Date   time.Time `json:"date"`
}

func AdaptCitationDto(citation models.Citation) CitationDto {
	return CitationDto{
		Title:  citation.Title,
		Domain: citation.Domain,
		Url:    citation.Url,
		Date:   citation.Date,
	}
}

type KYCEnrichmentResultDto struct {
	Analysis   string        `json:"analysis"`
	EntityName string        `json:"entity_name"`
	Citations  []CitationDto `json:"citations"`
}

func AdaptKYCEnrichmentResultDto(result models.AiEnrichmentKYC) KYCEnrichmentResultDto {
	return KYCEnrichmentResultDto{
		Analysis:   result.Analysis,
		EntityName: result.EntityName,
		Citations:  pure_utils.Map(result.Citations, AdaptCitationDto),
	}
}

type KYCEnrichmentResultsDto struct {
	Results []KYCEnrichmentResultDto `json:"results"`
}

func AdaptKYCEnrichmentResultsDto(results []models.AiEnrichmentKYC) KYCEnrichmentResultsDto {
	return KYCEnrichmentResultsDto{
		Results: pure_utils.Map(results, AdaptKYCEnrichmentResultDto),
	}
}
