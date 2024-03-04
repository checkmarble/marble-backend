package models

import (
	"time"

	"github.com/guregu/null/v5"
)

type Scenario struct {
	Id                     string
	CreatedAt              time.Time
	DecisionToCaseOutcomes []Outcome
	DecisionToCaseInboxId  *string
	Description            string
	LiveVersionID          *string
	Name                   string
	OrganizationId         string
	TriggerObjectType      string
}

type CreateScenarioInput struct {
	Description       string
	Name              string
	TriggerObjectType string
}

type UpdateScenarioInput struct {
	Id                     string
	DecisionToCaseOutcomes []Outcome
	DecisionToCaseInboxId  null.String
	Description            *string
	Name                   *string
}

type ListAllScenariosFilters struct {
	Live bool
}

type ScenarioAndIteration struct {
	Scenario  Scenario
	Iteration ScenarioIteration
}
