package dto

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
)

type DecisionFilters struct {
	CaseIds               []string  `form:"case_id[]"`
	CaseInboxIds          []string  `form:"case_inbox_id[]"`
	EndDate               time.Time `form:"end_date"`
	HasCase               *bool     `form:"has_case"`
	Outcomes              []string  `form:"outcome[]"`
	PivotValue            *string   `form:"pivot_value"`
	ReviewStatuses        []string  `form:"review_status[]"`
	ScenarioIds           []string  `form:"scenario_id[]"`
	ScheduledExecutionIds []string  `form:"scheduled_execution_id[]"`
	StartDate             time.Time `form:"start_date"`
	TriggerObjects        []string  `form:"trigger_object[]"`
	TriggerObjectId       *string   `form:"trigger_object_id"`

	// COMPAT: set to true to not error out if a scenario ID filter is passed that does not match
	// a scenario of the organization. Legacy APIs used to have a 400 returned.
	AllowInvalidScenarioId bool `form:"-"`
}

type DecisionListPageWithIndexesDto struct {
	Items       []Decision `json:"items"`
	StartIndex  int        `json:"start_index"`
	EndIndex    int        `json:"end_index"`
	HasNextPage bool       `json:"has_next_page"`
}

func AdaptDecisionListPageWithIndexesDto(decisionsPage models.DecisionListPageWithIndexes, marbleAppUrl *url.URL) DecisionListPageWithIndexesDto {
	// initialize as a non nil slice, so that it is serialized as an empty array instead of null
	items := make([]Decision, len(decisionsPage.Decisions))
	for i, decision := range decisionsPage.Decisions {
		items[i] = NewDecisionDto(decision, marbleAppUrl)
	}

	return DecisionListPageWithIndexesDto{
		Items:       items,
		StartIndex:  decisionsPage.StartIndex,
		EndIndex:    decisionsPage.EndIndex,
		HasNextPage: decisionsPage.HasNextPage,
	}
}

type DecisionListPageDto struct {
	Items       []Decision `json:"items"`
	HasNextPage bool       `json:"has_next_page"`
}

func AdaptDecisionListPageDto(decisionsPage models.DecisionListPage, marbleAppUrl *url.URL) DecisionListPageDto {
	items := make([]Decision, len(decisionsPage.Decisions))
	for i, decision := range decisionsPage.Decisions {
		items[i] = NewDecisionDto(decision, marbleAppUrl)
	}

	return DecisionListPageDto{
		Items:       items,
		HasNextPage: decisionsPage.HasNextPage,
	}
}

type CreateDecisionBody struct {
	TriggerObject json.RawMessage `json:"trigger_object" binding:"required"`
	ObjectType    string          `json:"object_type" binding:"required"`
}

type CreateDecisionWithScenarioBody struct {
	ScenarioId    string          `json:"scenario_id" binding:"required"`
	TriggerObject json.RawMessage `json:"trigger_object" binding:"required"`
	ObjectType    string          `json:"object_type" binding:"required"`
}

type CreateDecisionInput struct {
	Body *CreateDecisionBody `in:"body=json"`
}

type DecisionRule struct {
	Description   string    `json:"description"`
	Error         *ErrorDto `json:"error,omitempty"`
	Name          string    `json:"name"`
	Outcome       string    `json:"outcome"`
	Result        bool      `json:"result"`
	RuleId        string    `json:"rule_id"`
	ScoreModifier int       `json:"score_modifier"`
	ErrorCode     *int      `json:"error_code"`

	// RuleEvaluation is not returned by default, it only is for endpoints consumed by the frontend
	RuleEvaluation *ast.NodeEvaluationDto `json:"rule_evaluation,omitempty"`
}

type DecisionScreening struct {
	Id      string `json:"id"`
	Status  string `json:"status"`
	Partial bool   `json:"partial"`
	Count   int    `json:"count"`
}

type ErrorDto struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type DecisionScenario struct {
	Id                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	ScenarioIterationId uuid.UUID `json:"scenario_iteration_id"`
	Version             int       `json:"version"`
}

type PivotValue struct {
	PivotValue null.String `json:"pivot_value"`
	PivotId    *uuid.UUID  `json:"pivot_id"`
}

type Decision struct {
	Id                   uuid.UUID        `json:"id"`
	AppLink              null.String      `json:"app_link"`
	Case                 *APICase         `json:"case,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
	TriggerObject        map[string]any   `json:"trigger_object"`
	TriggerObjectType    string           `json:"trigger_object_type"`
	Outcome              string           `json:"outcome"`
	PivotValues          []PivotValue     `json:"pivot_values"`
	ReviewStatus         *string          `json:"review_status"`
	Scenario             DecisionScenario `json:"scenario"`
	Score                int              `json:"score"`
	ScheduledExecutionId *string          `json:"scheduled_execution_id"`
}

type DecisionWithRules struct {
	Decision
	Rules      []DecisionRule      `json:"rules"`
	Screenings []DecisionScreening `json:"sanction_checks,omitempty"` //nolint:tagliatelle
}

func NewDecisionDto(decision models.Decision, marbleAppUrl *url.URL) Decision {
	decisionDto := Decision{
		Id:                decision.DecisionId,
		AppLink:           toDecisionUrl(marbleAppUrl, decision.DecisionId.String()),
		CreatedAt:         decision.CreatedAt,
		TriggerObjectType: decision.ClientObject.TableName,
		TriggerObject:     decision.ClientObject.Data,
		Outcome:           decision.Outcome.String(),
		PivotValues:       make([]PivotValue, 0, 1),
		ReviewStatus:      decision.ReviewStatus,
		Scenario: DecisionScenario{
			Id:                  decision.ScenarioId,
			Name:                decision.ScenarioName,
			Description:         decision.ScenarioDescription,
			ScenarioIterationId: decision.ScenarioIterationId,
			Version:             decision.ScenarioVersion,
		},
		Score:                decision.Score,
		ScheduledExecutionId: decision.ScheduledExecutionId,
	}

	if decision.Case != nil {
		c := AdaptCaseDto(*decision.Case)
		decisionDto.Case = &c
	}

	if decision.PivotValue != nil {
		decisionDto.PivotValues = append(decisionDto.PivotValues, PivotValue{
			PivotId:    decision.PivotId,
			PivotValue: null.StringFromPtr(decision.PivotValue),
		})
	}

	return decisionDto
}

func toDecisionUrl(marbleAppUrl *url.URL, decisionId string) null.String {
	if marbleAppUrl == nil {
		return null.StringFrom("")
	}
	if marbleAppUrl.String() == "" {
		return null.StringFrom("")
	}

	url := url.URL{
		Scheme: marbleAppUrl.Scheme,
		Host:   marbleAppUrl.Host,
		Path:   fmt.Sprintf("/decisions/%s", decisionId),
	}
	return null.StringFrom(url.String())
}

func NewDecisionWithRuleDto(decision models.DecisionWithRuleExecutions, marbleAppUrl *url.URL, withRuleExecution bool) DecisionWithRules {
	decisionDto := DecisionWithRules{
		Decision: NewDecisionDto(decision.Decision, marbleAppUrl),
		Rules:    make([]DecisionRule, len(decision.RuleExecutions)),
	}

	for i, ruleExecution := range decision.RuleExecutions {
		decisionDto.Rules[i] = NewDecisionRuleDto(ruleExecution, withRuleExecution)
	}

	if decision.ScreeningExecutions != nil {
		decisionDto.Screenings = make([]DecisionScreening, len(decision.ScreeningExecutions))

		for idx, sce := range decision.ScreeningExecutions {
			decisionDto.Screenings[idx] = DecisionScreening{
				Id:      sce.Id,
				Status:  sce.Status.String(),
				Partial: sce.Partial,
				Count:   sce.Count,
			}
		}
	}

	return decisionDto
}

func NewDecisionRuleDto(rule models.RuleExecution, withRuleExecution bool) DecisionRule {
	out := DecisionRule{
		Name:          rule.Rule.Name,
		Description:   rule.Rule.Description,
		Outcome:       rule.Outcome,
		ScoreModifier: rule.ResultScoreModifier,
		Result:        rule.Result,
		RuleId:        rule.Rule.Id,
		Error:         ErrorDtoFromError(rule.ExecutionError),
	}
	if withRuleExecution {
		out.RuleEvaluation = rule.Evaluation
	}
	return out
}

func ErrorDtoFromError(execErr ast.ExecutionError) *ErrorDto {
	return &ErrorDto{
		Code:    int(execErr),
		Message: execErr.String(),
	}
}

type DecisionsAggregateMetadata struct {
	Count struct {
		Total          int `json:"total"`
		Approve        int `json:"approve"`
		Review         int `json:"review"`
		BlockAndReview int `json:"block_and_review"`
		Decline        int `json:"decline"`
		Skipped        int `json:"skipped"`
	} `json:"count"`
}
type DecisionsWithMetadata struct {
	Decisions []DecisionWithRules        `json:"decisions"`
	Metadata  DecisionsAggregateMetadata `json:"metadata"`
}

func AdaptDecisionsWithMetadataDto(
	decisions []models.DecisionWithRuleExecutions,
	marbleAppUrl *url.URL,
	nbSkipped int,
	withRuleExecution bool,
) DecisionsWithMetadata {
	decisionDtos := make([]DecisionWithRules, len(decisions))
	for i, decision := range decisions {
		decisionDtos[i] = NewDecisionWithRuleDto(decision, marbleAppUrl, withRuleExecution)
	}

	return DecisionsWithMetadata{
		Decisions: decisionDtos,
		Metadata:  AdaptDecisionsMetadata(decisions, nbSkipped),
	}
}

func AdaptDecisionsMetadata(
	decisions []models.DecisionWithRuleExecutions,
	nbSkipped int,
) DecisionsAggregateMetadata {
	metadata := DecisionsAggregateMetadata{}
	for _, decision := range decisions {
		switch decision.Outcome {
		case models.Approve:
			metadata.Count.Approve++
		case models.Review:
			metadata.Count.Review++
		case models.BlockAndReview:
			metadata.Count.BlockAndReview++
		case models.Decline:
			metadata.Count.Decline++
		}
	}
	metadata.Count.Total = len(decisions)
	metadata.Count.Skipped = nbSkipped
	return metadata
}
