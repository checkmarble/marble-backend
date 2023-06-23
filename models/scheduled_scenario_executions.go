package models

import "time"

type ScheduledExecution struct {
	ID                  string
	OrganizationId      string
	ScenarioID          string
	ScenarioIterationID string
	Status              ScheduledExecutionStatus
	StartedAt           time.Time
	FinishedAt          *time.Time
}

type ScheduledExecutionStatus int

const (
	ScheduledExecutionPending ScheduledExecutionStatus = iota
	ScheduledExecutionSuccess
	ScheduledExecutionFailure
)

func (s ScheduledExecutionStatus) String() string {
	switch s {
	case ScheduledExecutionPending:
		return "pending"
	case ScheduledExecutionSuccess:
		return "success"
	case ScheduledExecutionFailure:
		return "failure"
	}
	return "pending"
}

func ScheduledExecutionStatusFrom(s string) ScheduledExecutionStatus {
	switch s {
	case "pending":
		return ScheduledExecutionPending
	case "success":
		return ScheduledExecutionSuccess
	case "failure":
		return ScheduledExecutionFailure
	}
	return ScheduledExecutionPending
}

type UpdateScheduledExecutionInput struct {
	ID     string
	Status *ScheduledExecutionStatus
}

type CreateScheduledExecutionInput struct {
	OrganizationID      string
	ScenarioID          string
	ScenarioIterationID string
}
