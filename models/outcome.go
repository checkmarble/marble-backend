package models

type Outcome int

const (
	Approve Outcome = iota
	Review
	BlockAndReview
	Reject
	None
	UnknownOutcome
)

var ValidOutcomes = []Outcome{Approve, Review, BlockAndReview, Reject}

// Provide a string value for each outcome
func (o Outcome) String() string {
	switch o {
	case Approve:
		return "approve"
	case Review:
		return "review"
	case BlockAndReview:
		return "block_and_review"
	case Reject:
		return "decline"
	case None:
		return "null"
	case UnknownOutcome:
		return "unknown"
	}
	return "unknown"
}

// Provide an Outcome from a string value
func OutcomeFrom(s string) Outcome {
	switch s {
	case "approve":
		return Approve
	case "review":
		return Review
	case "block_and_review":
		return BlockAndReview
	case "decline":
		return Reject
	case "null":
		return None
	case "unknown":
		return UnknownOutcome
	}
	return UnknownOutcome
}
