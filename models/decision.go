package models

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

const (
	DECISION_TIMEOUT            = 10 * time.Second
	SEQUENTIAL_DECISION_TIMEOUT = 30 * time.Second
)

// Decision models
type Decision struct {
	DecisionId           string
	OrganizationId       string
	Case                 *Case
	CreatedAt            time.Time
	ClientObject         ClientObject
	Outcome              Outcome
	PivotId              *string
	PivotValue           *string
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
	PivotId             *string
	PivotValue          *string
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

func AdaptScenarExecToDecision(scenarioExecution ScenarioExecution, clientObject ClientObject, scheduledExecutionId *string) DecisionWithRuleExecutions {
	return DecisionWithRuleExecutions{
		Decision: Decision{
			DecisionId:           pure_utils.NewPrimaryKey(scenarioExecution.OrganizationId),
			ClientObject:         clientObject,
			Outcome:              scenarioExecution.Outcome,
			OrganizationId:       scenarioExecution.OrganizationId,
			PivotId:              scenarioExecution.PivotId,
			PivotValue:           scenarioExecution.PivotValue,
			ScenarioDescription:  scenarioExecution.ScenarioDescription,
			ScenarioId:           scenarioExecution.ScenarioId,
			ScenarioIterationId:  scenarioExecution.ScenarioIterationId,
			ScenarioName:         scenarioExecution.ScenarioName,
			ScenarioVersion:      scenarioExecution.ScenarioVersion,
			ScheduledExecutionId: scheduledExecutionId,
			Score:                scenarioExecution.Score,
		},
		RuleExecutions: scenarioExecution.RuleExecutions,
	}
}

// Decision input models
type CreateDecisionInput struct {
	OrganizationId     string
	PayloadRaw         json.RawMessage
	ClientObject       *ClientObject
	ScenarioId         string
	TriggerObjectTable string
}

type CreateAllDecisionsInput struct {
	OrganizationId     string
	PayloadRaw         json.RawMessage
	TriggerObjectTable string
}

type DecisionFilters struct {
	CaseIds               []string
	EndDate               time.Time
	HasCase               *bool
	Outcomes              []Outcome
	PivotValue            *string
	ScenarioIds           []string
	ScheduledExecutionIds []string
	StartDate             time.Time
	TriggerObjects        []string
}

const (
	DecisionSortingCreatedAt SortingField = "created_at"
)

type DecisionWorkflowFilters struct {
	InboxId        string
	OrganizationId string
	PivotValue     string
}
