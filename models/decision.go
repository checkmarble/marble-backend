package models

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

const (
	DECISION_TIMEOUT = 10 * time.Second
)

// Decision models
type Decision struct {
	DecisionId     uuid.UUID
	OrganizationId uuid.UUID
	Case           *Case
	CreatedAt      time.Time
	ClientObject   ClientObject
	Outcome        Outcome
	PivotId        *uuid.UUID
	PivotValue     *string
	ReviewStatus   *string
	ScenarioId     uuid.UUID
	ScenarioName   string

	// Deprecated. Remove it from the model after we remove the v0 publicAPI.
	ScenarioDescription  string
	ScenarioVersion      int
	Score                int
	ScheduledExecutionId *string
	ScenarioIterationId  uuid.UUID
}

const (
	ReviewStatusPending = "pending"
	ReviewStatusDecline = "decline"
	ReviewStatusApprove = "approve"
)

var ValidReviewStatuses = []string{ReviewStatusPending, ReviewStatusDecline, ReviewStatusApprove}

type DecisionMetadata struct {
	DecisionId     uuid.UUID
	OrganizationId uuid.UUID
	CreatedAt      time.Time
	Score          int
}

type DecisionWithRulesAndScreeningsBaseInfo struct {
	Decision

	// Rule executions should not be expected to contain the rule evaluation in this context
	RuleExecutions      []RuleExecution
	ScreeningExecutions []ScreeningBaseInfo
}

type DecisionWithRuleExecutions struct {
	Decision
	RuleExecutions      []RuleExecution
	ScreeningExecutions []ScreeningWithMatches
}

type DecisionsByVersionByOutcome struct {
	Version string
	Outcome string
	Score   int
	Count   int
}

type ScenarioExecution struct {
	ScenarioId          uuid.UUID
	ScenarioIterationId uuid.UUID
	ScenarioName        string

	// Deprecated. Remove it from the model after we remove the v0 publicAPI.
	ScenarioDescription string
	ScenarioVersion     int
	PivotId             *uuid.UUID
	PivotValue          *string
	RuleExecutions      []RuleExecution
	ScreeningExecutions []ScreeningWithMatches
	Score               int
	Outcome             Outcome
	OrganizationId      uuid.UUID
	TestRunId           string

	ExecutionMetrics *ScenarioExecutionMetrics
}

type ScenarioExecutionMetrics struct {
	Steps map[string]int64
	Rules map[string]int64
}

type RuleExecutionStat struct {
	Version      string
	Name         string
	Outcome      string
	StableRuleId *string
	Total        int
}

type RuleExecution struct {
	Id                  string
	DecisionId          string
	ExecutionError      ast.ExecutionError
	Evaluation          *ast.NodeEvaluationDto
	Outcome             string // enum: hit, no_hit, snoozed, error
	Result              bool
	ResultScoreModifier int
	Rule                Rule
	Duration            time.Duration
}

func AdaptScenarExecToDecision(scenarioExecution ScenarioExecution, clientObject ClientObject, scheduledExecutionId *string) DecisionWithRuleExecutions {
	var reviewStatus *string
	if scenarioExecution.Outcome == BlockAndReview {
		val := ReviewStatusPending
		reviewStatus = &val
	}

	decisionId := uuid.Must(uuid.NewV7())
	return DecisionWithRuleExecutions{
		Decision: Decision{
			DecisionId:           decisionId,
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
		RuleExecutions: scenarioExecution.RuleExecutions,
		ScreeningExecutions: pure_utils.Map(scenarioExecution.ScreeningExecutions,
			MergeScreeningExecWithDefaults(decisionId, scenarioExecution.OrganizationId)),
	}
}

func MergeScreeningExecWithDefaults(decisionId, orgId uuid.UUID) func(se ScreeningWithMatches) ScreeningWithMatches {
	return func(se ScreeningWithMatches) ScreeningWithMatches {
		if se.Id == "" {
			se.Id = uuid.Must(uuid.NewV7()).String()
		}
		se.DecisionId = decisionId.String()
		se.OrgId = orgId
		se.CreatedAt = time.Now()
		se.UpdatedAt = time.Now()
		return se
	}
}

type OffloadDecisionRuleRequest struct {
	OrgId           uuid.UUID
	DeleteBefore    time.Time
	BatchSize       int
	Watermark       *Watermark
	LargeInequality bool
}

type OffloadableDecisionRule struct {
	// Decision
	DecisionId string
	CreatedAt  time.Time

	// Rule execution
	RuleExecutionId *string
	RuleId          *string
	RuleOutcome     *string
	RuleEvaluation  *ast.NodeEvaluationDto
}

// Decision input models
type CreateDecisionInput struct {
	OrganizationId     uuid.UUID
	PayloadRaw         json.RawMessage
	ClientObject       *ClientObject
	ScenarioId         string
	TriggerObjectTable string
}

type CreateDecisionParams struct {
	WithDecisionWebhooks        bool
	WithRuleExecutionDetails    bool
	WithScenarioPermissionCheck bool
	WithDisallowUnknownFields   bool
	ConcurrentRules             int
}

type CreateAllDecisionsInput struct {
	OrganizationId     uuid.UUID
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
	ScenarioIds           []uuid.UUID
	ScheduledExecutionIds []string
	StartDate             time.Time
	TriggerObjects        []string
	TriggerObjectId       *string
}

type DecisionListPage struct {
	Decisions   []Decision
	HasNextPage bool
}

const DecisionSortingCreatedAt SortingField = SortingFieldCreatedAt

type DecisionWorkflowFilters struct {
	InboxId        *uuid.UUID
	OrganizationId uuid.UUID
	PivotValue     string
}
