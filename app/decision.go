package app

import (
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
	case "reject":
		return Reject
	case "null":
		return None
	case "unknown":
		return UnknownOutcome
	}
	return UnknownOutcome
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
	ID                  string
	CreatedAt           time.Time
	Payload             Payload
	Outcome             Outcome
	ScenarioID          string
	ScenarioName        string
	ScenarioDescription string
	ScenarioVersion     int
	RuleExecutions      []RuleExecution
	Score               int
	DecisionError       DecisionError
}
