package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/guregu/null/v5"
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
	DecisionToCaseOutcomes []string    `json:"decision_to_case_outcomes"`
	DecisionToCaseInboxId  null.String `json:"decision_to_case_inbox_id"`
	Description            *string     `json:"description"`
	Name                   *string     `json:"name"`
}

func AdaptUpdateScenario(scenarioId string, input UpdateScenarioBody) models.UpdateScenarioInput {
	parsedInput := models.UpdateScenarioInput{
		Id:                    scenarioId,
		DecisionToCaseInboxId: input.DecisionToCaseInboxId,
		Description:           input.Description,
		Name:                  input.Name,
	}
	if input.DecisionToCaseOutcomes != nil {
		parsedInput.DecisionToCaseOutcomes = pure_utils.Map(input.DecisionToCaseOutcomes, models.OutcomeFrom)
	}
	return parsedInput
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

type CreateScenarioIterationInput struct {
	Payload *CreateScenarioIterationBody `in:"body=json"`
}

type CreateDraftFromScenarioIterationInput struct {
	ScenarioIterationId string `in:"path=scenarioIterationId"`
}

func AdaptCreateScenario(input CreateScenarioBody) models.CreateScenarioInput {
	return models.CreateScenarioInput{
		Name:              input.Name,
		Description:       input.Description,
		TriggerObjectType: input.TriggerObjectType,
	}
}
