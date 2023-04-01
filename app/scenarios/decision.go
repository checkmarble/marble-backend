package scenarios

import (
	payload_package "marble/marble-backend/app/payload"
	"time"
)

// /////////////////////////////
// Outcomes
// /////////////////////////////
type Outcome int

const (
	Approve Outcome = iota
	Review
	Reject
	None
	Unknown
)

// Provide a string value for each outcome
func (o Outcome) String() string {
	switch o {
	case Approve:
		return "approve"
	case Review:
		return "review"
	case Reject:
		return "reject"
	case None:
		return "null"
	case Unknown:
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
	case "reject":
		return Reject
	case "null":
		return None
	case "unknown":
		return Unknown
	}
	return Unknown
}

///////////////////////////////
// Decision errors
///////////////////////////////

type DecisionError int

const (
	NoScoreAllRulesFailed DecisionError = 100
)

func (d DecisionError) String() string {
	switch d {
	case NoScoreAllRulesFailed:
		return "Scenario was not able to compute a score because all rules failed."
	}
	return ""
}

///////////////////////////////
// Decision
///////////////////////////////

type Decision struct {
	ID         string
	Created_at time.Time
	Payload    payload_package.Payload

	Outcome             Outcome
	ScenarioID          string
	ScenarioName        string
	ScenarioDescription string
	ScenarioVersion     int
	RuleExecutions      []RuleExecution
	Score               int

	DecisionError DecisionError
}
