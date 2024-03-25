package models

import (
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
