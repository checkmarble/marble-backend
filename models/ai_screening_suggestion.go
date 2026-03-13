package models

import "time"

type ScreeningHitConfidence string

const (
	ScreeningHitConfidenceProbableFalsePositive ScreeningHitConfidence = "probable_false_positive"
	// TODO: Define a better name for Neutral confidence level
	ScreeningHitConfidenceNeutral     ScreeningHitConfidence = "neutral"
	ScreeningHitConfidenceInvestigate ScreeningHitConfidence = "investigate"
)

var ScreeningHitConfidenceLevels = []ScreeningHitConfidence{
	ScreeningHitConfidenceProbableFalsePositive,
	ScreeningHitConfidenceNeutral,
	ScreeningHitConfidenceInvestigate,
}

type AiScreeningHitSuggestion struct {
	MatchId    string                 `json:"match_id"`
	EntityId   string                 `json:"entity_id"`
	Confidence ScreeningHitConfidence `json:"confidence"`
	Reason     string                 `json:"reason"`
	CreatedAt  time.Time              `json:"created_at"`
}
