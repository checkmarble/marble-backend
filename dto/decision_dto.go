package dto

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
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
}

type DecisionListPageWithIndexesDto struct {
	Items       []Decision `json:"items"`
	StartIndex  int        `json:"start_index"`
	EndIndex    int        `json:"end_index"`
	HasNextPage bool       `json:"has_next_page"`
}

func AdaptDecisionListPageWithIndexesDto(decisionsPage models.DecisionListPageWithIndexes, marbleAppHost string) DecisionListPageWithIndexesDto {
	// initialize as a non nil slice, so that it is serialized as an empty array instead of null
	items := make([]Decision, len(decisionsPage.Decisions))
	for i, decision := range decisionsPage.Decisions {
		items[i] = NewDecisionDto(decision, marbleAppHost)
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

func AdaptDecisionListPageDto(decisionsPage models.DecisionListPage, marbleAppHost string) DecisionListPageDto {
	items := make([]Decision, len(decisionsPage.Decisions))
	for i, decision := range decisionsPage.Decisions {
		items[i] = NewDecisionDto(decision, marbleAppHost)
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

type DecisionSanctionCheck struct {
	Partial bool `json:"partial"`
	Matches int  `json:"matches"`
}

type ErrorDto struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type DecisionScenario struct {
	Id                  string `json:"id"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	ScenarioIterationId string `json:"scenario_iteration_id"`
	Version             int    `json:"version"`
}

type PivotValue struct {
	PivotValue null.String `json:"pivot_value"`
	PivotId    null.String `json:"pivot_id"`
}

type Decision struct {
	Id                   string           `json:"id"`
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
	Rules         []DecisionRule         `json:"rules"`
	SanctionCheck *DecisionSanctionCheck `json:"sanction_check,omitempty"`
}

func NewDecisionDto(decision models.Decision, marbleAppHost string) Decision {
	decisionDto := Decision{
		Id:                decision.DecisionId,
		AppLink:           toDecisionUrl(marbleAppHost, decision.DecisionId),
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
			PivotId:    null.StringFromPtr(decision.PivotId),
			PivotValue: null.StringFromPtr(decision.PivotValue),
		})
	}

	return decisionDto
}

func toDecisionUrl(marbleAppHost string, decisionId string) null.String {
	if marbleAppHost == "" {
		return null.String{}
	}

	url := url.URL{
		Scheme: "https",
		Host:   marbleAppHost,
		Path:   fmt.Sprintf("/decisions/%s", decisionId),
	}
	return null.StringFrom(url.String())
}

func NewDecisionWithRuleDto(decision models.DecisionWithRuleExecutions, marbleAppHost string, withRuleExecution bool) DecisionWithRules {
	decisionDto := DecisionWithRules{
		Decision: NewDecisionDto(decision.Decision, marbleAppHost),
		Rules:    make([]DecisionRule, len(decision.RuleExecutions)),
	}

	for i, ruleExecution := range decision.RuleExecutions {
		decisionDto.Rules[i] = NewDecisionRuleDto(ruleExecution, withRuleExecution)
	}

	if decision.SanctionCheckExecution != nil {
		decisionDto.SanctionCheck = &DecisionSanctionCheck{
			Partial: decision.SanctionCheckExecution.Partial,
			Matches: decision.SanctionCheckExecution.Matches,
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
		Error:         ErrorDtoFromError(rule.Error),
	}
	if withRuleExecution {
		out.RuleEvaluation = rule.Evaluation
	}
	return out
}

func ErrorDtoFromError(err error) *ErrorDto {
	if err == nil {
		return nil
	}

	return &ErrorDto{
		Code:    int(ast.AdaptExecutionError(err)),
		Message: err.Error(),
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
	marbleAppHost string,
	nbSkipped int,
	withRuleExecution bool,
) DecisionsWithMetadata {
	decisionDtos := make([]DecisionWithRules, len(decisions))
	for i, decision := range decisions {
		decisionDtos[i] = NewDecisionWithRuleDto(decision, marbleAppHost, withRuleExecution)
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
