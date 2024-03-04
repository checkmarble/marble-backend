package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/guregu/null/v5"
)

// Read DTO
type ScenarioDto struct {
	Id                     string    `json:"id"`
	CreatedAt              time.Time `json:"createdAt"`
	DecisionToCaseOutcomes []string  `json:"decision_to_case_outcomes"`
	DecisionToCaseInboxId  string    `json:"decision_to_case_inbox_id"`
	Description            string    `json:"description"`
	LiveVersionID          *string   `json:"liveVersionId,omitempty"`
	Name                   string    `json:"name"`
	OrganizationId         string    `json:"organization_id"`
	TriggerObjectType      string    `json:"triggerObjectType"`
}

func AdaptScenarioDto(scenario models.Scenario) ScenarioDto {
	out := ScenarioDto{
		Id:        scenario.Id,
		CreatedAt: scenario.CreatedAt,
		DecisionToCaseOutcomes: pure_utils.Map(scenario.DecisionToCaseOutcomes,
			func(o models.Outcome) string { return o.String() }),
		Description:       scenario.Description,
		LiveVersionID:     scenario.LiveVersionID,
		Name:              scenario.Name,
		OrganizationId:    scenario.OrganizationId,
		TriggerObjectType: scenario.TriggerObjectType,
	}
	if scenario.DecisionToCaseInboxId != nil {
		out.DecisionToCaseInboxId = *scenario.DecisionToCaseInboxId
	}
	return out
}

// Create scenario DTO
type CreateScenarioBody struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TriggerObjectType string `json:"triggerObjectType"`
}

func AdaptCreateScenarioInput(input CreateScenarioBody) models.CreateScenarioInput {
	return models.CreateScenarioInput{
		Name:              input.Name,
		Description:       input.Description,
		TriggerObjectType: input.TriggerObjectType,
	}
}

// Update scenario DTO
type UpdateScenarioBody struct {
	DecisionToCaseOutcomes []string    `json:"decision_to_case_outcomes"`
	DecisionToCaseInboxId  null.String `json:"decision_to_case_inbox_id"`
	Description            *string     `json:"description"`
	Name                   *string     `json:"name"`
}

func AdaptUpdateScenarioInput(scenarioId string, input UpdateScenarioBody) models.UpdateScenarioInput {
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
