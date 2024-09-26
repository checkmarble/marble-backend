package models

import (
	"fmt"
	"time"
)

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
	Manual                   bool
}

type ScheduledExecutionStatus int

const (
	ScheduledExecutionPending ScheduledExecutionStatus = iota
	ScheduledExecutionProcessing
	ScheduledExecutionSuccess
	ScheduledExecutionFailure
)

func (s ScheduledExecutionStatus) String() string {
	switch s {
	case ScheduledExecutionPending:
		return "pending"
	case ScheduledExecutionProcessing:
		return "processing"
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
	case "processing":
		return ScheduledExecutionProcessing
	}
	return ScheduledExecutionPending
}

type UpdateScheduledExecutionStatusInput struct {
	Id                       string
	Status                   ScheduledExecutionStatus
	NumberOfCreatedDecisions *int
	CurrentStatusCondition   ScheduledExecutionStatus // Used for optimistic locking
}

type CreateScheduledExecutionInput struct {
	OrganizationId      string
	ScenarioId          string
	ScenarioIterationId string
	Manual              bool
}

type ListScheduledExecutionsFilters struct {
	OrganizationId string
	ScenarioId     string
	Status         []ScheduledExecutionStatus
	ExcludeManual  bool
}

type Filter struct {
	LeftSql    string
	LeftValue  any
	Operator   string
	RightSql   string
	RightValue any
}

func (f Filter) ToSql() (string, []any) {
	var args []any
	var left string
	if f.LeftSql != "" {
		left = f.LeftSql
	} else {
		left = "?"
		args = append(args, f.LeftValue)
	}

	var right string
	if f.RightSql != "" {
		right = f.RightSql
	} else {
		right = "?"
		args = append(args, f.RightValue)
	}

	return fmt.Sprintf("%s %s %s", left, f.Operator, right), args
}

type TableIdentifier struct {
	Schema string
	Table  string
}
