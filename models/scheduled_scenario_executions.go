package models

import "time"

type ScheduledExecution struct {
	Id                       string
	OrganizationId           string
	ScenarioId               string
	ScenarioIterationId      string
	Status                   ScheduledExecutionStatus
	StartedAt                time.Time
	FinishedAt               *time.Time
	NumberOfCreatedDecisions int
	Scenario                 Scenario
	Manual                   *bool
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
	Id                       string
	Status                   *ScheduledExecutionStatus
	NumberOfCreatedDecisions *int
}

type CreateScheduledExecutionInput struct {
	OrganizationId      string
	ScenarioId          string
	ScenarioIterationId string
}
