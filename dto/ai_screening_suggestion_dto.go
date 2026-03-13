package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type AiScreeningHitSuggestionDto struct {
	MatchId    string `json:"match_id"`
	EntityId   string `json:"entity_id"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func AdaptAiScreeningHitSuggestionDto(suggestion models.AiScreeningHitSuggestion) AiScreeningHitSuggestionDto {
	return AiScreeningHitSuggestionDto{
		MatchId:    suggestion.MatchId,
		EntityId:   suggestion.EntityId,
		Confidence: string(suggestion.Confidence),
		Reason:     suggestion.Reason,
		CreatedAt:  suggestion.CreatedAt,
	}
}

func AdaptAiScreeningHitSuggestionDtos(suggestions []models.AiScreeningHitSuggestion) []AiScreeningHitSuggestionDto {
	dtos := make([]AiScreeningHitSuggestionDto, len(suggestions))
	for i, s := range suggestions {
		dtos[i] = AdaptAiScreeningHitSuggestionDto(s)
	}
	return dtos
}
