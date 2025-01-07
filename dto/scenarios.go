package dto

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/guregu/null/v5"
)

// Read DTO
type ScenarioDto struct {
	Id                         string      `json:"id"`
	CreatedAt                  time.Time   `json:"created_at"`
	DecisionToCaseOutcomes     []string    `json:"decision_to_case_outcomes"`
	DecisionToCaseInboxId      null.String `json:"decision_to_case_inbox_id"`
	DecisionToCaseWorkflowType string      `json:"decision_to_case_workflow_type"`
	DecisionToCaseNameTemplate *NodeDto    `json:"decision_to_case_name_template"`
	Description                string      `json:"description"`
	LiveVersionID              *string     `json:"live_version_id,omitempty"`
	Name                       string      `json:"name"`
	OrganizationId             string      `json:"organization_id"`
	TriggerObjectType          string      `json:"trigger_object_type"`
}

func AdaptScenarioDto(scenario models.Scenario) (ScenarioDto, error) {
	scenarioDto := ScenarioDto{
		Id:                    scenario.Id,
		CreatedAt:             scenario.CreatedAt,
		DecisionToCaseInboxId: null.StringFromPtr(scenario.DecisionToCaseInboxId),
		DecisionToCaseOutcomes: pure_utils.Map(scenario.DecisionToCaseOutcomes,
			func(o models.Outcome) string { return o.String() }),
		DecisionToCaseWorkflowType: string(scenario.DecisionToCaseWorkflowType),
		Description:                scenario.Description,
		LiveVersionID:              scenario.LiveVersionID,
		Name:                       scenario.Name,
		OrganizationId:             scenario.OrganizationId,
		TriggerObjectType:          scenario.TriggerObjectType,
	}

	if scenario.DecisionToCaseNameTemplate != nil {
		astDto, err := AdaptNodeDto(*scenario.DecisionToCaseNameTemplate)
		if err != nil {
			return ScenarioDto{},
				fmt.Errorf("unable to marshal ast expression: %w", err)
		}
		scenarioDto.DecisionToCaseNameTemplate = &astDto
	}

	return scenarioDto, nil
}

// Create scenario DTO
type CreateScenarioBody struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TriggerObjectType string `json:"trigger_object_type"`
}

func AdaptCreateScenarioInput(input CreateScenarioBody, organizationId string) models.CreateScenarioInput {
	out := models.CreateScenarioInput{
		Name:              input.Name,
		Description:       input.Description,
		TriggerObjectType: input.TriggerObjectType,
		OrganizationId:    organizationId,
	}

	return out
}

// Update scenario DTO
type UpdateScenarioBody struct {
	DecisionToCaseOutcomes     []string    `json:"decision_to_case_outcomes"`
	DecisionToCaseInboxId      null.String `json:"decision_to_case_inbox_id"`
	DecisionToCaseWorkflowType *string     `json:"decision_to_case_workflow_type"`
	DecisionToCaseNameTemplate *NodeDto    `json:"decision_to_case_name_template"`
	Description                *string     `json:"description"`
	Name                       *string     `json:"name"`
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
	if input.DecisionToCaseWorkflowType != nil {
		val := models.WorkflowType(*input.DecisionToCaseWorkflowType)
		parsedInput.DecisionToCaseWorkflowType = &val
	}
	if input.DecisionToCaseNameTemplate != nil {
		astNode, err := AdaptASTNode(*input.DecisionToCaseNameTemplate)
		if err == nil {
			parsedInput.DecisionToCaseNameTemplate = &astNode
		}
	}
	return parsedInput
}
