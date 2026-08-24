package models

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowType string

const (
	WorkflowDisabled            WorkflowType = "DISABLED"
	WorkflowCreateCase          WorkflowType = "CREATE_CASE"
	WorkflowAddToCaseIfPossible WorkflowType = "ADD_TO_CASE_IF_POSSIBLE"
)

var ValidWorkflowTypes = []WorkflowType{
	WorkflowDisabled,
	WorkflowCreateCase,
	WorkflowAddToCaseIfPossible,
}

func WorkflowTypeFromString(s string) WorkflowType {
	switch s {
	case "ADD_TO_CASE_IF_POSSIBLE":
		return WorkflowAddToCaseIfPossible
	case "CREATE_CASE":
		return WorkflowCreateCase
	default:
		return WorkflowDisabled
	}
}

type Scenario struct {
	Id                string
	CreatedAt         time.Time
	Description       string
	LiveVersionID     *string
	Name              string
	OrganizationId    uuid.UUID
	TriggerObjectType string
	Archived          bool

	// When set, batch executions create at most one decision per data object for this
	// scenario, keyed on (scenario, object_id) and persisted in scenario_scored_objects.
	// Enabling it on an existing scenario does not backfill, so the first run after the
	// toggle scores everything once. Only honoured under the BATCH_EXECUTION_V2 flag.
	DeduplicateBatchObjects bool
}

type CreateScenarioInput struct {
	Description       string
	Name              string
	TriggerObjectType string
	OrganizationId    uuid.UUID
}

type UpdateScenarioInput struct {
	Id                      string
	Description             *string
	Name                    *string
	Archived                *bool
	DeduplicateBatchObjects *bool
}

type ListAllScenariosFilters struct {
	Live           bool
	OrganizationId *uuid.UUID
}

type ScenarioAndIteration struct {
	Scenario  Scenario
	Iteration ScenarioIteration
}

type ScenarioRuleLatestVersion struct {
	Type              string
	StableId          string
	Name              string
	LatestVersion     string
	ScreeningProvider ScreeningProvider
}
