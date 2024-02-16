package models

import "time"

type Scenario struct {
	Id                string
	OrganizationId    string
	Name              string
	Description       string
	TriggerObjectType string
	CreatedAt         time.Time
	LiveVersionID     *string
}

type CreateScenarioInput struct {
	Name              string
	Description       string
	TriggerObjectType string
}

type UpdateScenarioInput struct {
	Id          string
	Name        *string
	Description *string
}

type ListAllScenariosFilters struct {
	Live bool
}

type ScenarioAndIteration struct {
	Scenario  Scenario
	Iteration ScenarioIteration
}
