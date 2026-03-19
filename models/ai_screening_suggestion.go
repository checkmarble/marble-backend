package models

type ScreeningHitConfidence string

const (
	ScreeningHitConfidenceProbableFalsePositive ScreeningHitConfidence = "probable_false_positive"
	ScreeningHitConfidenceInconclusive ScreeningHitConfidence = "inconclusive"
	ScreeningHitConfidenceInvestigate ScreeningHitConfidence = "investigate"
)

func (c ScreeningHitConfidence) IsValid() bool {
	switch c {
	case ScreeningHitConfidenceProbableFalsePositive,
		ScreeningHitConfidenceInconclusive,
		ScreeningHitConfidenceInvestigate:
		return true
	default:
		return false
	}
}
