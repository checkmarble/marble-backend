package dto

import (
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
	ScenarioId string              `in:"path=scenarioId"`
	Body       *UpdateScenarioBody `in:"body=json"`
}

// Scenario iterations

type UpdateScenarioIterationBody struct {
	Body *struct {
		TriggerConditionAstExpression *NodeDto `json:"trigger_condition_ast_expression"`
		ScoreReviewThreshold          *int     `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold          *int     `json:"scoreRejectThreshold,omitempty"`
		Schedule                      *string  `json:"schedule"`
		BatchTriggerSQL               *string  `json:"batchTriggerSQL"`
	} `json:"body,omitempty"`
}

type UpdateScenarioIterationInput struct {
	ScenarioIterationId string                       `in:"path=scenarioIterationId"`
	Payload             *UpdateScenarioIterationBody `in:"body=json"`
}

type CreateScenarioIterationBody struct {
	ScenarioId string `json:"scenarioId"`
	Body       *struct {
		TriggerConditionAstExpression *NodeDto              `json:"trigger_condition_ast_expression"`
		Rules                         []CreateRuleInputBody `json:"rules"`
		ScoreReviewThreshold          *int                  `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold          *int                  `json:"scoreRejectThreshold,omitempty"`
		Schedule                      string                `json:"schedule"`
		BatchTriggerSQL               string                `json:"batchTriggerSQL"`
	} `json:"body,omitempty"`
}

type CreateScenarioIterationInput struct {
	Payload *CreateScenarioIterationBody `in:"body=json"`
}

type CreateDraftFromScenarioIterationInput struct {
	ScenarioIterationId string `in:"path=scenarioIterationId"`
}

// scenario publications

type CreateScenarioPublicationBody struct {
	ScenarioIterationId string `json:"scenarioIterationID"`
	PublicationAction   string `json:"publicationAction"`
}

type CreateScenarioPublicationInput struct {
	Body *CreateScenarioPublicationBody `in:"body=json"`
}

type ListScenarioPublicationsInput struct {
	ScenarioId          string `in:"query=scenarioID"`
	ScenarioIterationId string `in:"query=scenarioIterationID"`
}

func AdaptCreateScenario(input *CreateScenarioInput, organizationId string) models.CreateScenarioInput {
	return models.CreateScenarioInput{
		OrganizationId:    organizationId,
		Name:              input.Body.Name,
		Description:       input.Body.Description,
		TriggerObjectType: input.Body.TriggerObjectType,
	}
}
