package models

type Outcome int

const (
	Approve Outcome = iota
	Review
	BlockAndReview
	Decline
	UnknownOutcome

	UnsetForcedOutcome = -1
)

var (
	ValidOutcomes      = []Outcome{Approve, Review, BlockAndReview, Decline}
	ValidForcedOutcome = []Outcome{Review, BlockAndReview, Decline, UnsetForcedOutcome}
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

func (o *Outcome) MaybeString() *string {
	if o == nil {
		return nil
	}

	value := "unknown"

	switch *o {
	case Approve:
		value = "approve"
	case Review:
		value = "review"
	case BlockAndReview:
		value = "block_and_review"
	case Decline:
		value = "decline"
	}

	return &value
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

func ForcedOutcomeFrom(s string) Outcome {
	if s == "none" {
		return UnsetForcedOutcome
	}

	return OutcomeFrom(s)
}
