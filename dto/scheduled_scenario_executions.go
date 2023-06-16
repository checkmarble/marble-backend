package dto

import (
	"marble/marble-backend/models"
	"time"
)

type APIScheduledExecution struct {
	ID                  string     `json:"id"`
	ScenarioIterationID string     `json:"scenario_iteration_id"`
	Status              string     `json:"status"`
	StartedAt           time.Time  `json:"started_at"`
	FinishedAt          *time.Time `json:"finished_at"`
}

func AdaptScheduledExecutionDto(ExecutionBatch models.ScheduledExecution) APIScheduledExecution {
	return APIScheduledExecution{
		ID:                  ExecutionBatch.ID,
		ScenarioIterationID: ExecutionBatch.ScenarioIterationID,
		Status:              ExecutionBatch.Status,
		StartedAt:           ExecutionBatch.StartedAt,
		FinishedAt:          ExecutionBatch.FinishedAt,
	}
}

type ListScheduledExecutionInput struct {
	ScenarioID string `in:"query=scenario_id;required"`
}
