package dto

import (
	"marble/marble-backend/models"
	"time"
)

type APIDecisionRule struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	ScoreModifier int       `json:"score_modifier"`
	Result        bool      `json:"result"`
	Error         *APIError `json:"error"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type APIDecisionScenario struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}

type APIDecision struct {
	ID                string              `json:"id"`
	CreatedAt         time.Time           `json:"created_at"`
	TriggerObject     map[string]any      `json:"trigger_object"`
	TriggerObjectType string              `json:"trigger_object_type"`
	Outcome           string              `json:"outcome"`
	Scenario          APIDecisionScenario `json:"scenario"`
	Rules             []APIDecisionRule   `json:"rules"`
	Score             int                 `json:"score"`
	Error             *APIError           `json:"error"`
}

func NewAPIDecision(decision models.Decision) APIDecision {
	apiDecision := APIDecision{
		ID:                decision.ID,
		CreatedAt:         decision.CreatedAt,
		TriggerObjectType: decision.ClientObject.TableName,
		TriggerObject:     decision.ClientObject.Data,
		Outcome:           decision.Outcome.String(),
		Scenario: APIDecisionScenario{
			ID:          decision.ScenarioID,
			Name:        decision.ScenarioName,
			Description: decision.ScenarioDescription,
			Version:     decision.ScenarioVersion,
		},
		Score: decision.Score,
		Rules: make([]APIDecisionRule, len(decision.RuleExecutions)),
	}

	for i, ruleExecution := range decision.RuleExecutions {
		apiDecision.Rules[i] = NewAPIDecisionRule(ruleExecution)
	}

	// Error added here to make sure it does not appear if empty
	// Otherwise, by default it will generate an empty APIError{}
	if int(decision.DecisionError) != 0 {
		apiDecision.Error = &APIError{int(decision.DecisionError), decision.DecisionError.String()}
	}

	return apiDecision
}

func NewAPIDecisionRule(rule models.RuleExecution) APIDecisionRule {
	apiDecisionRule := APIDecisionRule{
		Name:          rule.Rule.Name,
		Description:   rule.Rule.Description,
		ScoreModifier: rule.ResultScoreModifier,
		Result:        rule.Result,
	}

	// Error added here to make sure it does not appear if empty
	// Otherwise, by default it will generate an empty APIError{}
	if int(rule.Error) != 0 {
		apiDecisionRule.Error = &APIError{int(rule.Error), rule.Error.String()}
	}

	return apiDecisionRule
}
