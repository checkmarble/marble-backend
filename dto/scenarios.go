package dto

import (
	"encoding/json"
	"marble/marble-backend/models"
)

type CreateScenarioBody struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TriggerObjectType string `json:"triggerObjectType"`
}

type CreateScenarioInput struct {
	Body *CreateScenarioBody `in:"body=json"`
}

type UpdateScenarioBody struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type UpdateScenarioInput struct {
	ScenarioID string              `in:"path=scenarioID"`
	Body       *UpdateScenarioBody `in:"body=json"`
}

// Scenario iterations

type UpdateScenarioIterationBody struct {
	Body *struct {
		TriggerCondition     *json.RawMessage `json:"triggerCondition,omitempty"`
		ScoreReviewThreshold *int             `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold *int             `json:"scoreRejectThreshold,omitempty"`
		Schedule             *string          `json:"schedule"`
		BatchTriggerSQL      *string          `json:"batchTriggerSQL"`
	} `json:"body,omitempty"`
}

type UpdateScenarioIterationInput struct {
	ScenarioIterationID string                       `in:"path=scenarioIterationID"`
	Payload             *UpdateScenarioIterationBody `in:"body=json"`
}

type CreateScenarioIterationBody struct {
	ScenarioID string `json:"scenarioId"`
	Body       *struct {
		TriggerCondition     *json.RawMessage                       `json:"triggerCondition,omitempty"`
		Rules                []CreateScenarioIterationRuleInputBody `json:"rules"`
		ScoreReviewThreshold *int                                   `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold *int                                   `json:"scoreRejectThreshold,omitempty"`
		Schedule             string                                 `json:"schedule"`
		BatchTriggerSQL      string                                 `json:"batchTriggerSQL"`
	} `json:"body,omitempty"`
}

type CreateScenarioIterationInput struct {
	Payload *CreateScenarioIterationBody `in:"body=json"`
}

// rules

type CreateScenarioIterationRuleInputBody struct {
	ScenarioIterationID  string          `json:"scenarioIterationId"`
	DisplayOrder         int             `json:"displayOrder"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Formula              json.RawMessage `json:"formula"`
	FormulaAstExpression *NodeDto        `json:"formula_ast_expression"`
	ScoreModifier        int             `json:"scoreModifier"`
}

type CreateScenarioIterationRuleInput struct {
	Body *CreateScenarioIterationRuleInputBody `in:"body=json"`
}

type UpdateScenarioIterationRuleBody struct {
	DisplayOrder         *int             `json:"displayOrder,omitempty"`
	Name                 *string          `json:"name,omitempty"`
	Description          *string          `json:"description,omitempty"`
	Formula              *json.RawMessage `json:"formula,omitempty"`
	FormulaAstExpression *NodeDto         `json:"formula_ast_expression"`
	ScoreModifier        *int             `json:"scoreModifier,omitempty"`
}

type UpdateScenarioIterationRuleInput struct {
	RuleID string                           `in:"path=ruleID"`
	Body   *UpdateScenarioIterationRuleBody `in:"body=json"`
}

// scenario publications

type CreateScenarioPublicationBody struct {
	ScenarioIterationID string `json:"scenarioIterationID"`
	PublicationAction   string `json:"publicationAction"`
}

type CreateScenarioPublicationInput struct {
	Body *CreateScenarioPublicationBody `in:"body=json"`
}

type ListScenarioPublicationsInput struct {
	ScenarioID          string `in:"query=scenarioID"`
	ScenarioIterationID string `in:"query=scenarioIterationID"`
}

func AdaptCreateScenario(input *CreateScenarioInput, orgID string) models.CreateScenarioInput {
	return models.CreateScenarioInput{
		OrganizationID:    orgID,
		Name:              input.Body.Name,
		Description:       input.Body.Description,
		TriggerObjectType: input.Body.TriggerObjectType,
	}
}
