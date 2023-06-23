package models

import "time"

type Decision struct {
	DecisionId           string
	OrganizationId       string
	CreatedAt            time.Time
	ClientObject         ClientObject
	Outcome              Outcome
	ScenarioId           string
	ScenarioName         string
	ScenarioDescription  string
	ScenarioVersion      int
	RuleExecutions       []RuleExecution
	Score                int
	DecisionError        DecisionError
	ScheduledExecutionId *string
}

type ScenarioExecution struct {
	ScenarioID          string
	ScenarioName        string
	ScenarioDescription string
	ScenarioVersion     int
	RuleExecutions      []RuleExecution
	Score               int
	Outcome             Outcome
}

type RuleExecution struct {
	Rule                Rule
	Result              bool
	ResultScoreModifier int
	Error               RuleExecutionError
}

type RuleExecutionError int

const (
	DivisionByZero RuleExecutionError = 100
	NullFieldRead  RuleExecutionError = 200
	NoRowsRead     RuleExecutionError = 201
)

func (r RuleExecutionError) String() string {
	switch r {
	case DivisionByZero:
		return "A division by zero occurred in a rule"
	case NullFieldRead:
		return "A field read in a rule is null"
	case NoRowsRead:
		return "No rows were read from db in a rule"
	}
	return ""
}
