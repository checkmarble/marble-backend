package models

import (
	"errors"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
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
	Evaluation          *ast.NodeEvaluationDto
	ResultScoreModifier int
	Error               error
}

type ExecutionError int

const (
	NoError              ExecutionError = 0
	DivisionByZero       ExecutionError = 100
	NullFieldRead        ExecutionError = 200
	NoRowsRead           ExecutionError = 201
	PayloadFieldNotFound ExecutionError = 202
	Unknown              ExecutionError = -1
)

func (r ExecutionError) String() string {
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

func AdaptExecutionError(err error) ExecutionError {
	switch {
	case err == nil:
		return NoError
	case errors.Is(err, ast.ErrNullFieldRead):
		return NullFieldRead
	case errors.Is(err, ast.ErrNoRowsRead):
		return NoRowsRead
	case errors.Is(err, ast.ErrDivisionByZero):
		return DivisionByZero
	case errors.Is(err, ast.ErrPayloadFieldNotFound):
		return PayloadFieldNotFound
	default:
		return Unknown
	}
}
