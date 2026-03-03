package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AsyncDecisionExecutionStatus string

const (
	AsyncDecisionExecution_Pending   AsyncDecisionExecutionStatus = "pending"
	AsyncDecisionExecution_Ingested  AsyncDecisionExecutionStatus = "ingested"
	AsyncDecisionExecution_Completed AsyncDecisionExecutionStatus = "completed"
	AsyncDecisionExecution_Failed    AsyncDecisionExecutionStatus = "failed"
)

type AsyncDecisionExecutionFailureStage string

const (
	AsyncDecisionExecution_StageIngestion AsyncDecisionExecutionFailureStage = "ingestion"
	AsyncDecisionExecution_StageDecision  AsyncDecisionExecutionFailureStage = "decision"
)

type AsyncDecisionExecution struct {
	Id            uuid.UUID
	OrgId         uuid.UUID
	ObjectType    string
	TriggerObject json.RawMessage
	ScenarioId    *string
	ShouldIngest  bool
	Status        AsyncDecisionExecutionStatus
	DecisionIds   []uuid.UUID
	ErrorMessage  *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AsyncDecisionExecutionCreate struct {
	Id            uuid.UUID
	OrgId         uuid.UUID
	ObjectType    string
	TriggerObject json.RawMessage
	ScenarioId    *string
	ShouldIngest  bool
}

type AsyncDecisionExecutionUpdate struct {
	Id           uuid.UUID
	Status       AsyncDecisionExecutionStatus
	DecisionIds  []uuid.UUID
	ErrorMessage *string
}
