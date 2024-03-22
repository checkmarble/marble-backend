package models

import (
	"errors"
	"time"
)

type Decision struct {
	DecisionId           string
	OrganizationId       string
	Case                 *Case
	CreatedAt            time.Time
	ClientObject         ClientObject
	Outcome              Outcome
	ScenarioId           string
	ScenarioName         string
	ScenarioDescription  string
	ScenarioVersion      int
	Score                int
	ScheduledExecutionId *string
	ScenarioIterationId  string
}

type DecisionCore struct {
	DecisionId     string
	OrganizationId string
	CreatedAt      time.Time
	Score          int
}

type DecisionWithRuleExecutions struct {
	Decision
	RuleExecutions []RuleExecution
}

type DecisionWithRank struct {
	Decision
	RankNumber int
	TotalCount TotalCount
}

type ScenarioExecution struct {
	ScenarioId          string
	ScenarioIterationId string
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
	Error               error
}

type RuleExecutionError int

const (
	NoError              RuleExecutionError = 0
	DivisionByZero       RuleExecutionError = 100
	NullFieldRead        RuleExecutionError = 200
	NoRowsRead           RuleExecutionError = 201
	PayloadFieldNotFound RuleExecutionError = 202
	Unknown              RuleExecutionError = -1
)

func (r RuleExecutionError) String() string {
	switch r {
	case DivisionByZero:
		return "A division by zero occurred in a rule"
	case NullFieldRead:
		return "A field read in a rule is null"
	case NoRowsRead:
		return "No rows were read from db in a rule"
	case PayloadFieldNotFound:
		return "A payload field was not found in a rule"
	case Unknown:
		return "Unknown error"
	}
	return ""
}

func AdaptRuleExecutionError(err error) RuleExecutionError {
	switch {
	case err == nil:
		return NoError
	case errors.Is(err, ErrNullFieldRead):
		return NullFieldRead
	case errors.Is(err, ErrNoRowsRead):
		return NoRowsRead
	case errors.Is(err, ErrDivisionByZero):
		return DivisionByZero
	case errors.Is(err, ErrPayloadFieldNotFound):
		return PayloadFieldNotFound
	default:
		return Unknown
	}
}
