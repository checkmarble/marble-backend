package models

import "time"

type Scenario struct {
	ID                string
	OrganizationID    string
	Name              string
	Description       string
	TriggerObjectType string
	CreatedAt         time.Time
	LiveVersionID     *string
}

type CreateScenarioInput struct {
	OrganizationID    string
	Name              string
	Description       string
	TriggerObjectType string
}

type UpdateScenarioInput struct {
	ID          string
	Name        *string
	Description *string
}
