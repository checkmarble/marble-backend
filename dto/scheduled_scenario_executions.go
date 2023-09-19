package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APIScheduledExecution struct {
	Id                  string     `json:"id"`
	ScenarioIterationId string     `json:"scenario_iteration_id"`
	Status              string     `json:"status"`
	StartedAt           time.Time  `json:"started_at"`
	FinishedAt          *time.Time `json:"finished_at"`
}

func AdaptScheduledExecutionDto(ExecutionBatch models.ScheduledExecution) APIScheduledExecution {
	return APIScheduledExecution{
		Id:                  ExecutionBatch.Id,
		ScenarioIterationId: ExecutionBatch.ScenarioIterationId,
		Status:              ExecutionBatch.Status.String(),
		StartedAt:           ExecutionBatch.StartedAt,
		FinishedAt:          ExecutionBatch.FinishedAt,
	}
}

type ListScheduledExecutionInput struct {
	ScenarioId string `in:"query=scenario_id"`
}
