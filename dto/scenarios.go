package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/guregu/null/v5"
)

// Read DTO
type ScenarioDto struct {
	Id                         string      `json:"id"`
	CreatedAt_deprec           time.Time   //nolint:tagliatelle
	CreatedAt                  time.Time   `json:"created_at"`
	DecisionToCaseOutcomes     []string    `json:"decision_to_case_outcomes"`
	DecisionToCaseInboxId      null.String `json:"decision_to_case_inbox_id"`
	DecisionToCaseWorkflowType string      `json:"decision_to_case_workflow_type"`
	Description                string      `json:"description"`
	LiveVersionID_deprec       *string     `json:"liveVersionId,omitempty"` //nolint:tagliatelle
	LiveVersionID              *string     `json:"live_version_id,omitempty"`
	Name                       string      `json:"name"`
	OrganizationId             string      `json:"organization_id"`
	TriggerObjectType_deprec   string      `json:"triggerObjectType"` //nolint:tagliatelle
	TriggerObjectType          string      `json:"trigger_object_type"`
}

func AdaptScenarioDto(scenario models.Scenario) ScenarioDto {
	return ScenarioDto{
		Id:                    scenario.Id,
		CreatedAt_deprec:      scenario.CreatedAt,
		CreatedAt:             scenario.CreatedAt,
		DecisionToCaseInboxId: null.StringFromPtr(scenario.DecisionToCaseInboxId),
		DecisionToCaseOutcomes: pure_utils.Map(scenario.DecisionToCaseOutcomes,
			func(o models.Outcome) string { return o.String() }),
		DecisionToCaseWorkflowType: string(scenario.DecisionToCaseWorkflowType),
		Description:                scenario.Description,
		LiveVersionID_deprec:       scenario.LiveVersionID,
		LiveVersionID:              scenario.LiveVersionID,
		Name:                       scenario.Name,
		OrganizationId:             scenario.OrganizationId,
		TriggerObjectType_deprec:   scenario.TriggerObjectType,
		TriggerObjectType:          scenario.TriggerObjectType,
	}
}

// Create scenario DTO
type CreateScenarioBody struct {
	Name                     string `json:"name"`
	Description              string `json:"description"`
	TriggerObjectType_deprec string `json:"triggerObjectType"` //nolint:tagliatelle
	TriggerObjectType        string `json:"trigger_object_type"`
}

func AdaptCreateScenarioInput(input CreateScenarioBody, organizationId string) models.CreateScenarioInput {
	out := models.CreateScenarioInput{
		Name:              input.Name,
		Description:       input.Description,
		TriggerObjectType: input.TriggerObjectType,
		OrganizationId:    organizationId,
	}
	// TODO remove deprecated fields
	if out.TriggerObjectType == "" {
		out.TriggerObjectType = input.TriggerObjectType_deprec
	}

	return out
}

// Update scenario DTO
type UpdateScenarioBody struct {
	DecisionToCaseOutcomes     []string    `json:"decision_to_case_outcomes"`
	DecisionToCaseInboxId      null.String `json:"decision_to_case_inbox_id"`
	DecisionToCaseWorkflowType *string     `json:"decision_to_case_workflow_type"`
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
	return parsedInput
}
