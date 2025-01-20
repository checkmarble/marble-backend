package models

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
)

const (
	DECISION_TIMEOUT = 10 * time.Second
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
	ReviewStatus         *string
	ScenarioId           string
	ScenarioName         string
	ScenarioDescription  string
	ScenarioVersion      int
	Score                int
	ScheduledExecutionId *string
	ScenarioIterationId  string
}

const (
	ReviewStatusPending = "pending"
	ReviewStatusDecline = "decline"
	ReviewStatusApprove = "approve"
)

var ValidReviewStatuses = []string{ReviewStatusPending, ReviewStatusDecline, ReviewStatusApprove}

type DecisionCore struct {
	DecisionId     string
	OrganizationId string
	CreatedAt      time.Time
	Score          int
}

type DecisionWithRuleExecutions struct {
	Decision
	RuleExecutions         []RuleExecution
	SanctionCheckExecution *SanctionCheckExecution
}

type DecisionsByVersionByOutcome struct {
	Version string
	Outcome string
	Score   int
	Count   int
}

type DecisionWithRank struct {
	Decision
	RankNumber int
}

type ScenarioExecution struct {
	ScenarioId             string
	ScenarioIterationId    string
	ScenarioName           string
	ScenarioDescription    string
	ScenarioVersion        int
	PivotId                *string
	PivotValue             *string
	RuleExecutions         []RuleExecution
	SanctionCheckExecution *SanctionCheckExecution
	Score                  int
	Outcome                Outcome
	OrganizationId         string
	TestRunId              string
}

type RuleExecutionStat struct {
	Version      string
	Name         string
	Outcome      string
	StableRuleId *string
	Total        int
}

type RuleExecution struct {
	DecisionId          string
	Error               error
	Evaluation          *ast.NodeEvaluationDto
	Outcome             string // enum: hit, no_hit, snoozed, error
	Result              bool
	ResultScoreModifier int
	Rule                Rule
}

func AdaptScenarExecToDecision(scenarioExecution ScenarioExecution, clientObject ClientObject, scheduledExecutionId *string) DecisionWithRuleExecutions {
	var reviewStatus *string
	if scenarioExecution.Outcome == BlockAndReview {
		val := ReviewStatusPending
		reviewStatus = &val
	}

	return DecisionWithRuleExecutions{
		Decision: Decision{
			DecisionId:           uuid.Must(uuid.NewV7()).String(),
			CreatedAt:            time.Now(),
			ClientObject:         clientObject,
			Outcome:              scenarioExecution.Outcome,
			OrganizationId:       scenarioExecution.OrganizationId,
			PivotId:              scenarioExecution.PivotId,
			PivotValue:           scenarioExecution.PivotValue,
			ReviewStatus:         reviewStatus,
			ScenarioDescription:  scenarioExecution.ScenarioDescription,
			ScenarioId:           scenarioExecution.ScenarioId,
			ScenarioIterationId:  scenarioExecution.ScenarioIterationId,
			ScenarioName:         scenarioExecution.ScenarioName,
			ScenarioVersion:      scenarioExecution.ScenarioVersion,
			ScheduledExecutionId: scheduledExecutionId,
			Score:                scenarioExecution.Score,
		},
		RuleExecutions:         scenarioExecution.RuleExecutions,
		SanctionCheckExecution: scenarioExecution.SanctionCheckExecution,
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

type CreateDecisionParams struct {
	WithDecisionWebhooks        bool
	WithRuleExecutionDetails    bool
	WithScenarioPermissionCheck bool
}

type CreateAllDecisionsInput struct {
	OrganizationId     string
	PayloadRaw         json.RawMessage
	TriggerObjectTable string
}

type DecisionFilters struct {
	CaseIds               []string
	CaseInboxIds          []string
	EndDate               time.Time
	HasCase               *bool
	Outcomes              []Outcome
	PivotValue            *string
	ReviewStatuses        []string
	ScenarioIds           []string
	ScheduledExecutionIds []string
	StartDate             time.Time
	TriggerObjects        []string
}

type DecisionListPageWithIndexes struct {
	Decisions   []Decision
	StartIndex  int
	EndIndex    int
	HasNextPage bool
}

type DecisionListPage struct {
	Decisions   []Decision
	HasNextPage bool
}

const DecisionSortingCreatedAt SortingField = SortingFieldCreatedAt

type DecisionWorkflowFilters struct {
	InboxId        string
	OrganizationId string
	PivotValue     string
}
