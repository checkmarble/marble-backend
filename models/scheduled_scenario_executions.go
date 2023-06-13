package models

import "time"

type ScheduledExecution struct {
	ID                  string
	ScenarioIterationID string
	Status              string
	StartedAt           time.Time
	FinishedAt          *time.Time
}

type UpdateScheduledExecutionBody struct {
	Status *string
}
