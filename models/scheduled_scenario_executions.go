package models

import "time"

type ScheduledExecution struct {
	ID                  string
	ScenarioIterationID string
	Status              string
	StartedAt           time.Time
	FinishedAt          *time.Time
}

type UpdateScheduledExecutionInput struct {
	ID     string
	Status *string
}

type CreateScheduledExecutionInput struct {
	Organizationid      string
	ScenarioId          string
	ScenarioIterationID string
}
