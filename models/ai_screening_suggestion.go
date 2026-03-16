package models

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

func (c ScreeningHitConfidence) IsValid() bool {
	switch c {
	case ScreeningHitConfidenceProbableFalsePositive,
		ScreeningHitConfidenceNeutral,
		ScreeningHitConfidenceInvestigate:
		return true
	default:
		return false
	}
}
