package models

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type Workflow struct {
	WorkflowRule

	Conditions []WorkflowCondition
	Actions    []WorkflowAction
}

type WorkflowRule struct {
	Id         string
	ScenarioId string
	Name       string
	Priority   int

	CreatedAt time.Time
	UpdatedAt *time.Time
}

type WorkflowCondition struct {
	Id       string
	RuleId   string
	Function string
	Params   json.RawMessage

	CreatedAt time.Time
	UpdatedAt *time.Time
}

type WorkflowAction struct {
	Id     string
	RuleId string
	Action WorkflowType
	Params json.RawMessage

	CreatedAt time.Time
	UpdatedAt *time.Time
}

func ParseWorkflowAction[T any](action WorkflowAction) (WorkflowActionSpec[T], error) {
	out := WorkflowActionSpec[T]{Action: action.Action}

	switch action.Action {
	case WorkflowCreateCase, WorkflowAddToCaseIfPossible:
		if err := json.Unmarshal(action.Params, &out.Params); err != nil {
			return out, errors.Wrap(err, "could not unmarshal workflow action parameters")
		}

		return out, nil
	default:
		return WorkflowActionSpec[T]{Action: WorkflowDisabled}, nil
	}
}

type WorkflowActionSpec[T any] struct {
	Action WorkflowType
	Params T
}

type WorkflowCaseParams struct {
	InboxId       *uuid.UUID `json:"inbox_id"`
	TitleTemplate *ast.Node  `json:"title_template"`
}

type WorkflowExecution struct {
	AddedToCase bool
	WebhookIds  []string
}
