package models

import "time"

type ScheduledScenarioBatchExecution struct {
	ID                  string
	ScenarioID          string
	ScenarioIterationID string
	Status              string
	StartedAt           time.Time
	FinishedAt          *time.Time
}

type UpdateScheduledScenarioExecutionBody struct {
	Status *string
}

type ScheduledScenarioObjectExecution struct {
	ID                       string
	ScenarioID               string
	ScenarioIterationID      string
	ScenarioBatchExecutionID string
	TriggerObjectID          string
	TriggerObjectType        string
	Status                   string
}

type ScheduledScenarioObjectExecutionUpdateBody struct {
	Status *string
}
