package models

import (
	"time"
)

type Scenario struct {
	Id                string
	CreatedAt         time.Time
	Description       string
	LiveVersionID     *string
	Name              string
	OrganizationId    string
	TriggerObjectType string
}

type CreateScenarioInput struct {
	Description       string
	Name              string
	TriggerObjectType string
	OrganizationId    string
}

type UpdateScenarioInput struct {
	Id          string
	Description *string
	Name        *string
}

type ListAllScenariosFilters struct {
	Live bool
}

type ScenarioAndIteration struct {
	Scenario  Scenario
	Iteration ScenarioIteration
}

type ScenarioRuleLatestVersion struct {
	Type          string
	StableId      string
	Name          string
	LatestVersion string
}
