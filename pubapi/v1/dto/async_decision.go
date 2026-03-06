package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/google/uuid"
)

type AsyncDecisionExecution struct {
	Id           uuid.UUID      `json:"id"`
	ObjectType   string         `json:"object_type"`
	ScenarioId   *string        `json:"scenario_id"`
	Status       string         `json:"status"`
	DecisionIds  []uuid.UUID    `json:"decision_ids"`
	ErrorMessage *string        `json:"error_message"`
	CreatedAt    types.DateTime `json:"created_at"`
	UpdatedAt    types.DateTime `json:"updated_at"`
}

type AsyncDecisionExecutionCreated struct {
	Id uuid.UUID `json:"id"`
}

func AdaptAsyncDecisionExecution(m models.AsyncDecisionExecution) AsyncDecisionExecution {
	decisionIds := m.DecisionIds
	if decisionIds == nil {
		decisionIds = make([]uuid.UUID, 0)
	}
	return AsyncDecisionExecution{
		Id:           m.Id,
		ObjectType:   m.ObjectType,
		ScenarioId:   m.ScenarioId,
		Status:       m.Status.String(),
		DecisionIds:  decisionIds,
		ErrorMessage: m.ErrorMessage,
		CreatedAt:    types.DateTime(m.CreatedAt),
		UpdatedAt:    types.DateTime(m.UpdatedAt),
	}
}

func AdaptAsyncDecisionExecutionCreated(m models.AsyncDecisionExecution) AsyncDecisionExecutionCreated {
	return AsyncDecisionExecutionCreated{
		Id: m.Id,
	}
}

type CreateAsyncDecisionParams struct {
	TriggerObjectType string            `json:"trigger_object_type"`
	TriggerObjects    []json.RawMessage `json:"trigger_objects" binding:"required,min=1,max=100"`
	ScenarioId        *string           `json:"scenario_id" binding:"omitempty,uuid"`
	Ingest            bool              `json:"ingest"`
}
