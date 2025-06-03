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

// Priority returns which outcome should override another, in the case several executions return different outcome
// Higher priority will override lower.
func (o Outcome) Priority() int {
	switch o {
	case Approve:
		return 0
	case Review:
		return 1
	case BlockAndReview:
		return 2
	case Decline:
		return 3
	default:
		return -1
	}
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
