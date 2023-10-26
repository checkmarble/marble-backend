package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APIScheduledExecution struct {
	Id                        string     `json:"id"`
	ScenarioIterationId       string     `json:"scenario_iteration_id"`
	Status                    string     `json:"status"`
	StartedAt                 time.Time  `json:"started_at"`
	FinishedAt                *time.Time `json:"finished_at"`
	NumberOfCreatedDecisions  int        `json:"number_of_created_decisions"`
	ScenarioId                string     `json:"scenario_id"`
	ScenarioName              string     `json:"scenario_name"`
	ScenarioTriggerObjectType string     `json:"scenario_trigger_object_type"`
	Manual                    bool       `json:"manual"`
}

func AdaptScheduledExecutionDto(ExecutionBatch models.ScheduledExecution) APIScheduledExecution {
	return APIScheduledExecution{
		Id:                        ExecutionBatch.Id,
		ScenarioIterationId:       ExecutionBatch.ScenarioIterationId,
		Status:                    ExecutionBatch.Status.String(),
		StartedAt:                 ExecutionBatch.StartedAt,
		FinishedAt:                ExecutionBatch.FinishedAt,
		NumberOfCreatedDecisions:  ExecutionBatch.NumberOfCreatedDecisions,
		ScenarioId:                ExecutionBatch.ScenarioId,
		ScenarioName:              ExecutionBatch.Scenario.Name,
		ScenarioTriggerObjectType: ExecutionBatch.Scenario.TriggerObjectType,
		Manual:                    ExecutionBatch.Manual,
	}
}
