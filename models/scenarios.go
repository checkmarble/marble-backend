package models

import "time"

type Scenario struct {
	ID                string
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
	ID          string
	Name        *string
	Description *string
}
