package models

import (
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/guregu/null/v5"
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

type Scenario struct {
	Id                         string
	CreatedAt                  time.Time
	DecisionToCaseOutcomes     []Outcome
	DecisionToCaseInboxId      *string
	DecisionToCaseWorkflowType WorkflowType
	DecisionToCaseNameTemplate *ast.Node
	Description                string
	LiveVersionID              *string
	Name                       string
	OrganizationId             string
	TriggerObjectType          string
}

type CreateScenarioInput struct {
	Description       string
	Name              string
	TriggerObjectType string
	OrganizationId    string
}

type UpdateScenarioInput struct {
	Id                         string
	DecisionToCaseOutcomes     []Outcome
	DecisionToCaseInboxId      null.String
	DecisionToCaseWorkflowType *WorkflowType
	DecisionToCaseNameTemplate *ast.Node
	Description                *string
	Name                       *string
}

type ListAllScenariosFilters struct {
	Live bool
}

type ScenarioAndIteration struct {
	Scenario  Scenario
	Iteration ScenarioIteration
}
