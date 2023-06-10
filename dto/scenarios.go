package dto

import (
	"encoding/json"
	"marble/marble-backend/models"
)

type CreateScenarioBody struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TriggerObjectType string `json:"triggerObjectType"`
	ScenarioType      string `json:"scenarioType"`
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

type ListScenarioInput struct {
	ScenarioType *string `in:"query=scenario_type"`
	IsActive     *bool   `in:"query=is_active"`
}

func (input ListScenarioInput) ToFilters() models.ListScenariosFilters {
	output := models.ListScenariosFilters{}
	stringType := models.ScenarioTypeFrom(*input.ScenarioType)
	if input.ScenarioType != nil {
		output.ScenarioType = &stringType
	}
	output.IsActive = input.IsActive

	return output
}

// Scenario iterations

type UpdateScenarioIterationBody struct {
	Body *struct {
		TriggerCondition     *json.RawMessage `json:"triggerCondition,omitempty"`
		ScoreReviewThreshold *int             `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold *int             `json:"scoreRejectThreshold,omitempty"`
		Schedule             *string          `json:"schedule"`
		BatchTriggerSQL      *string          `json:"batchTriggerSQL"`
	} `json:"body,omtiempty"`
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
	ScenarioIterationID string          `json:"scenarioIterationId"`
	DisplayOrder        int             `json:"displayOrder"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	Formula             json.RawMessage `json:"formula"`
	ScoreModifier       int             `json:"scoreModifier"`
}

type CreateScenarioIterationRuleInput struct {
	Body *CreateScenarioIterationRuleInputBody `in:"body=json"`
}

type UpdateScenarioIterationRuleBody struct {
	DisplayOrder  *int             `json:"displayOrder,omitempty"`
	Name          *string          `json:"name,omitempty"`
	Description   *string          `json:"description,omitempty"`
	Formula       *json.RawMessage `json:"formula,omitempty"`
	ScoreModifier *int             `json:"scoreModifier,omitempty"`
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
	PublicationAction   string `in:"query=publicationAction"`
}
