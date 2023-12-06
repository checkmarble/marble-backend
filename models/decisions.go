package models

import "time"

type Outcome int

const (
	Approve Outcome = iota
	Review
	Reject
	None
	UnknownOutcome
)

// Provide a string value for each outcome
func (o Outcome) String() string {
	switch o {
	case Approve:
		return "approve"
	case Review:
		return "review"
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
	case "decline":
		return Reject
	case "null":
		return None
	case "unknown":
		return UnknownOutcome
	}
	return UnknownOutcome
}

type CreateDecisionInput struct {
	OrganizationId          string
	ScenarioId              string
	ClientObject            ClientObject
	PayloadStructWithReader PayloadReader
}

type DecisionFilters struct {
	ScenarioIds    []string
	StartDate      time.Time
	EndDate        time.Time
	Outcomes       []Outcome
	TriggerObjects []TableName
	WithCase       *bool
	CaseIds        []string
}

const (
	DecisionSortingCreatedAt SortingField = "created_at"
)
