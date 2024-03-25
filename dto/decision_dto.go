package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
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
	TriggerObjectRaw  json.RawMessage `json:"trigger_object"`
	TriggerObjectType string          `json:"object_type"`
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

func NewAPIDecision(decision models.Decision) APIDecision {
	apiDecision := APIDecision{
		Id:                decision.DecisionId,
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

func NewAPIDecisionWithRule(decision models.DecisionWithRuleExecutions) APIDecisionWithRules {
	apiDecision := APIDecisionWithRules{
		APIDecision: NewAPIDecision(decision.Decision),
		Rules:       make([]APIDecisionRule, len(decision.RuleExecutions)),
	}

	for i, ruleExecution := range decision.RuleExecutions {
		apiDecision.Rules[i] = NewAPIDecisionRule(ruleExecution)
	}

	return apiDecision
}

func NewAPIDecisionRule(rule models.RuleExecution) APIDecisionRule {
	apiDecisionRule := APIDecisionRule{
		Name:           rule.Rule.Name,
		Description:    rule.Rule.Description,
		ScoreModifier:  rule.ResultScoreModifier,
		Result:         rule.Result,
		RuleId:         rule.Rule.Id,
		RuleEvaluation: rule.Evaluation,
	}

	// Error added here to make sure it does not appear if empty
	// Otherwise, by default it will generate an empty APIError{}
	if rule.Error != nil {
		apiDecisionRule.Error = &APIError{1, rule.Error.Error()}
	}

	return apiDecisionRule
}
