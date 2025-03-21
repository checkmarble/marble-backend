package models

type Outcome int

const (
	Approve Outcome = iota
	Review
	BlockAndReview
	Decline
	UnknownOutcome
)

var (
	ValidOutcomes      = []Outcome{Approve, Review, BlockAndReview, Decline}
	ValidForcedOutcome = []Outcome{Review, BlockAndReview, Decline}
)

// Provide a string value for each outcome
func (o Outcome) String() string {
	switch o {
	case Approve:
		return "approve"
	case Review:
		return "review"
	case BlockAndReview:
		return "block_and_review"
	case Decline:
		return "decline"
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
		return Decline
	}
	return UnknownOutcome
}
