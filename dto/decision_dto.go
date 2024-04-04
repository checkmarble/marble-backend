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

type GetDecisionInput struct {
	DecisionId string `in:"path=decisionId"`
}

type DecisionFilters struct {
	ScenarioIds    []string  `form:"scenarioId[]"`
	StartDate      time.Time `form:"startDate" time_format`
	EndDate        time.Time `form:"endDate" time_format`
	Outcomes       []string  `form:"outcome[]"`
	TriggerObjects []string  `form:"triggerObject[]"`
	CaseIds        []string  `form:"caseId[]"`
	HasCase        *bool     `form:"has_case"`
}

type CreateDecisionBody struct {
	ScenarioId        string          `json:"scenario_id"`
	TriggerObjectRaw  json.RawMessage `json:"trigger_object" binding:"required"`
	TriggerObjectType string          `json:"object_type" binding:"required"`
}

type CreateDecisionInputDto struct {
	Body *CreateDecisionBody `in:"body=json"`
}

type APIDecisionRule struct {
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	ScoreModifier  int                    `json:"score_modifier"`
	Result         bool                   `json:"result"`
	Error          *APIError              `json:"error,omitempty"`
	RuleId         string                 `json:"rule_id"`
	RuleEvaluation *ast.NodeEvaluationDto `json:"rule_evaluation,omitempty"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type APIDecisionScenario struct {
	Id                  string `json:"id"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	ScenarioIterationId string `json:"scenario_iteration_id"`
	Version             int    `json:"version"`
}

type APIDecision struct {
	Id                   string              `json:"id"`
	AppLink              null.String         `json:"app_link"`
	Case                 *APICase            `json:"case,omitempty"`
	CreatedAt            time.Time           `json:"created_at"`
	TriggerObject        map[string]any      `json:"trigger_object"`
	TriggerObjectType    string              `json:"trigger_object_type"`
	Outcome              string              `json:"outcome"`
	Scenario             APIDecisionScenario `json:"scenario"`
	Score                int                 `json:"score"`
	ScheduledExecutionId *string             `json:"scheduled_execution_id,omitempty"`
}

type APIDecisionWithRules struct {
	APIDecision
	Rules []APIDecisionRule `json:"rules"`
}

func NewAPIDecision(decision models.Decision, marbleAppHost string) APIDecision {
	apiDecision := APIDecision{
		Id:                decision.DecisionId,
		AppLink:           toDecisionUrl(marbleAppHost, decision.DecisionId),
		CreatedAt:         decision.CreatedAt,
		TriggerObjectType: string(decision.ClientObject.TableName),
		TriggerObject:     decision.ClientObject.Data,
		Outcome:           decision.Outcome.String(),
		Scenario: APIDecisionScenario{
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
		apiDecision.Case = &c
	}

	return apiDecision
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

func NewAPIDecisionWithRule(decision models.DecisionWithRuleExecutions, marbleAppHost string) APIDecisionWithRules {
	apiDecision := APIDecisionWithRules{
		APIDecision: NewAPIDecision(decision.Decision, marbleAppHost),
		Rules:       make([]APIDecisionRule, len(decision.RuleExecutions)),
	}

	for i, ruleExecution := range decision.RuleExecutions {
		apiDecision.Rules[i] = NewAPIDecisionRule(ruleExecution)
	}

	return apiDecision
}

func NewAPIDecisionRule(rule models.RuleExecution) APIDecisionRule {
	return APIDecisionRule{
		Name:           rule.Rule.Name,
		Description:    rule.Rule.Description,
		ScoreModifier:  rule.ResultScoreModifier,
		Result:         rule.Result,
		RuleId:         rule.Rule.Id,
		RuleEvaluation: rule.Evaluation,
		Error:          APIErrorFromError(rule.Error),
	}
}

func APIErrorFromError(err error) *APIError {
	if err == nil {
		return nil
	}

	return &APIError{
		Code:    int(ast.AdaptExecutionError(err)),
		Message: err.Error(),
	}
}

type DecisionsAggregateMetadata struct {
	Count struct {
		Total   int `json:"total"`
		Approve int `json:"approve"`
		Review  int `json:"review"`
		Reject  int `json:"reject"`
		Skipped int `json:"skipped"`
	} `json:"count"`
}
type APIDecisionsWithMetadata struct {
	Decisions []APIDecisionWithRules     `json:"decisions"`
	Metadata  DecisionsAggregateMetadata `json:"metadata"`
}

func AdaptAPIDecisionsWithMetadata(
	decisions []models.DecisionWithRuleExecutions,
	marbleAppHost string,
	nbSkipped int,
) APIDecisionsWithMetadata {
	apiDecisions := make([]APIDecisionWithRules, len(decisions))
	for i, decision := range decisions {
		apiDecisions[i] = NewAPIDecisionWithRule(decision, marbleAppHost)
	}

	return APIDecisionsWithMetadata{
		Decisions: apiDecisions,
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
		case models.Reject:
			metadata.Count.Reject++
		}
	}
	metadata.Count.Total = len(decisions)
	metadata.Count.Skipped = nbSkipped
	return metadata
}
