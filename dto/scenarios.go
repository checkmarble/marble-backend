package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

// Read DTO
type ScenarioDto struct {
	Id                string    `json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	Description       string    `json:"description"`
	LiveVersionID     *string   `json:"live_version_id,omitempty"`
	Name              string    `json:"name"`
	OrganizationId    uuid.UUID `json:"organization_id"`
	TriggerObjectType string    `json:"trigger_object_type"`
}

func AdaptScenarioDto(scenario models.Scenario) (ScenarioDto, error) {
	scenarioDto := ScenarioDto{
		Id:                scenario.Id,
		CreatedAt:         scenario.CreatedAt,
		Description:       scenario.Description,
		LiveVersionID:     scenario.LiveVersionID,
		Name:              scenario.Name,
		OrganizationId:    scenario.OrganizationId,
		TriggerObjectType: scenario.TriggerObjectType,
	}

	return scenarioDto, nil
}

// Create scenario DTO
type CreateScenarioBody struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TriggerObjectType string `json:"trigger_object_type"`
}

func AdaptCreateScenarioInput(input CreateScenarioBody, organizationId uuid.UUID) models.CreateScenarioInput {
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
	Description *string `json:"description"`
	Name        *string `json:"name"`
}

func AdaptUpdateScenarioInput(scenarioId string, input UpdateScenarioBody) models.UpdateScenarioInput {
	parsedInput := models.UpdateScenarioInput{
		Id:          scenarioId,
		Description: input.Description,
		Name:        input.Name,
	}

	return parsedInput
}

type ScenarioRuleLatestVersion struct {
	Type          string `json:"type"`
	StableId      string `json:"stable_id"`
	Name          string `json:"name"`
	LatestVersion string `json:"latest_version"`
}

func AdaptScenarioRuleLatestVersion(rule models.ScenarioRuleLatestVersion) ScenarioRuleLatestVersion {
	return ScenarioRuleLatestVersion{
		Type:          rule.Type,
		StableId:      rule.StableId,
		Name:          rule.Name,
		LatestVersion: rule.LatestVersion,
	}
}
