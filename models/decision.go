package models

import (
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

const (
	DECISION_TIMEOUT            = 10 * time.Second
	SEQUENTIAL_DECISION_TIMEOUT = 30 * time.Second
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
	OrganizationId      string
}

type RuleExecution struct {
	DecisionId          string
	Rule                Rule
	Result              bool
	Evaluation          *ast.NodeEvaluationDto
	ResultScoreModifier int
	Error               error
}

func AdaptScenarExecToDecision(scenarioExecution ScenarioExecution, clientObject ClientObject) DecisionWithRuleExecutions {
	return DecisionWithRuleExecutions{
		Decision: Decision{
			DecisionId:          pure_utils.NewPrimaryKey(scenarioExecution.OrganizationId),
			ClientObject:        clientObject,
			Outcome:             scenarioExecution.Outcome,
			ScenarioDescription: scenarioExecution.ScenarioDescription,
			ScenarioId:          scenarioExecution.ScenarioId,
			ScenarioIterationId: scenarioExecution.ScenarioIterationId,
			ScenarioName:        scenarioExecution.ScenarioName,
			ScenarioVersion:     scenarioExecution.ScenarioVersion,
			Score:               scenarioExecution.Score,
		},
		RuleExecutions: scenarioExecution.RuleExecutions,
	}
}
