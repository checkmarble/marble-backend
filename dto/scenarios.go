package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type CreateScenarioBody struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TriggerObjectType string `json:"triggerObjectType"`
}

type UpdateScenarioBody struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// Scenario iterations
type UpdateScenarioIterationData struct {
	TriggerConditionAstExpression *NodeDto `json:"trigger_condition_ast_expression"`
	ScoreReviewThreshold          *int     `json:"scoreReviewThreshold,omitempty"`
	ScoreRejectThreshold          *int     `json:"scoreRejectThreshold,omitempty"`
	Schedule                      *string  `json:"schedule"`
	BatchTriggerSQL               *string  `json:"batchTriggerSQL"`
}

type UpdateScenarioIterationBody struct {
	Body *UpdateScenarioIterationData `json:"body,omitempty"`
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

// scenario publications

type CreateScenarioPublicationBody struct {
	ScenarioIterationId string `json:"scenarioIterationID"`
	PublicationAction   string `json:"publicationAction"`
}

func AdaptCreateScenario(input CreateScenarioBody) models.CreateScenarioInput {
	return models.CreateScenarioInput{
		Name:              input.Name,
		Description:       input.Description,
		TriggerObjectType: input.TriggerObjectType,
	}
}
