package dto

import (
	"marble/marble-backend/models"
	"time"
)

type APIScheduledScenarioBatchExecution struct {
	ID                  string     `json:"id"`
	ScenarioID          string     `json:"scenario_id"`
	ScenarioIterationID string     `json:"scenario_iteration_id"`
	Status              string     `json:"status"`
	StartedAt           time.Time  `json:"started_at"`
	FinishedAt          *time.Time `json:"finished_at"`
}

func AdaptBatchExecutionDto(ExecutionBatch models.ScheduledScenarioBatchExecution) APIScheduledScenarioBatchExecution {
	return APIScheduledScenarioBatchExecution{
		ID:                  ExecutionBatch.ID,
		ScenarioID:          ExecutionBatch.ScenarioID,
		ScenarioIterationID: ExecutionBatch.ScenarioIterationID,
		Status:              ExecutionBatch.Status,
		StartedAt:           ExecutionBatch.StartedAt,
		FinishedAt:          ExecutionBatch.FinishedAt,
	}
}

type UpdateScheduledScenarioExecutionInput struct {
	ID   string                                `in:"path=scheduledScenarioExecutionID;required"`
	Body *UpdateScheduledScenarioExecutionBody `in:"body=json;required"`
}

type UpdateScheduledScenarioExecutionBody struct {
	Status *string `json:"status"`
}

func AdaptBatchExecutionUpdateBody(input UpdateScheduledScenarioExecutionBody) models.UpdateScheduledScenarioExecutionBody {
	return models.UpdateScheduledScenarioExecutionBody{
		Status: input.Status,
	}
}

type ListScheduledScenarioExecutionInput struct {
	ScenarioID string `in:"query=scenarioID;required"`
}

type APIScheduledScenarioObjectExecution struct {
	ID                       string `json:"id"`
	ScenarioID               string `json:"scenario_id"`
	ScenarioIterationID      string `json:"scenario_iteration_id"`
	ScenarioBatchExecutionID string `json:"scenario_batch_execution_id"`
	TriggerObjectID          string `json:"trigger_object_id"`
	TriggerObjectType        string `json:"trigger_object_type"`
	Status                   string `json:"status"`
}

func AdaptBatchExecutionObjectDto(item models.ScheduledScenarioObjectExecution) APIScheduledScenarioObjectExecution {
	return APIScheduledScenarioObjectExecution{
		ID:                       item.ID,
		ScenarioID:               item.ScenarioID,
		ScenarioIterationID:      item.ScenarioIterationID,
		ScenarioBatchExecutionID: item.ScenarioBatchExecutionID,
		TriggerObjectID:          item.TriggerObjectID,
		TriggerObjectType:        item.TriggerObjectType,
		Status:                   item.Status,
	}
}

type ListScheduledScenarioObjectExecutionInput struct {
	ScenarioBatchExecutionID string `in:"query=scenarioBatchExecutionID;required"`
}
