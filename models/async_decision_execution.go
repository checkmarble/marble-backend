package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AsyncDecisionExecutionStatus int

const (
	AsyncDecisionExecutionStatusUnknown AsyncDecisionExecutionStatus = iota
	AsyncDecisionExecutionStatusPending
	AsyncDecisionExecutionStatusIngested
	AsyncDecisionExecutionStatusCompleted
	AsyncDecisionExecutionStatusFailed
)

func (s AsyncDecisionExecutionStatus) String() string {
	switch s {
	case AsyncDecisionExecutionStatusPending:
		return "pending"
	case AsyncDecisionExecutionStatusIngested:
		return "ingested"
	case AsyncDecisionExecutionStatusCompleted:
		return "completed"
	case AsyncDecisionExecutionStatusFailed:
		return "failed"
	}
	return "unknown"
}

func AsyncDecisionExecutionStatusFromString(s string) AsyncDecisionExecutionStatus {
	switch s {
	case "pending":
		return AsyncDecisionExecutionStatusPending
	case "ingested":
		return AsyncDecisionExecutionStatusIngested
	case "completed":
		return AsyncDecisionExecutionStatusCompleted
	case "failed":
		return AsyncDecisionExecutionStatusFailed
	default:
		return AsyncDecisionExecutionStatusUnknown
	}
}

type AsyncDecisionExecutionFailureStage int

const (
	AsyncDecisionExecutionStageUnknown AsyncDecisionExecutionFailureStage = iota
	AsyncDecisionExecutionStageIngestion
	AsyncDecisionExecutionStageDecision
)

func (s AsyncDecisionExecutionFailureStage) String() string {
	switch s {
	case AsyncDecisionExecutionStageIngestion:
		return "ingestion"
	case AsyncDecisionExecutionStageDecision:
		return "decision"
	}
	return "unknown"
}

func AsyncDecisionExecutionFailureStageFromString(s string) AsyncDecisionExecutionFailureStage {
	switch s {
	case "ingestion":
		return AsyncDecisionExecutionStageIngestion
	case "decision":
		return AsyncDecisionExecutionStageDecision
	default:
		return AsyncDecisionExecutionStageUnknown
	}
}

type AsyncDecisionExecution struct {
	Id            uuid.UUID
	OrgId         uuid.UUID
	ObjectType    string
	TriggerObject json.RawMessage
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
	Status       *AsyncDecisionExecutionStatus
	DecisionIds  *[]uuid.UUID
	ErrorMessage *string
}
