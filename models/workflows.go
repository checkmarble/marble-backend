package models

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type Workflow struct {
	WorkflowRule

	Conditions []WorkflowCondition
	Actions    []WorkflowAction
}

type WorkflowRule struct {
	Id          uuid.UUID
	ScenarioId  uuid.UUID
	Name        string
	Priority    int
	Fallthrough bool

	CreatedAt time.Time
	UpdatedAt *time.Time
}

type WorkflowConditionType string

const (
	WorkflowConditionUnknown   WorkflowConditionType = "unknown"
	WorkflowConditionAlways    WorkflowConditionType = "always"
	WorkflowConditionNever     WorkflowConditionType = "never"
	WorkflowConditionOutcomeIn WorkflowConditionType = "outcome_in"
	WorkflowConditionRuleHit   WorkflowConditionType = "rule_hit"
	WorkflowPayloadEvaluates   WorkflowConditionType = "payload_evaluates"
)

var ValidWorkflowConditions = []WorkflowConditionType{
	WorkflowConditionAlways,
	WorkflowConditionNever,
	WorkflowConditionOutcomeIn,
	WorkflowConditionRuleHit,
}

func (t WorkflowConditionType) String() string {
	return string(t)
}

func WorkflowConditionFromString(s string) WorkflowConditionType {
	switch s {
	case WorkflowConditionAlways.String():
		return WorkflowConditionAlways
	case WorkflowConditionNever.String():
		return WorkflowConditionNever
	case WorkflowConditionOutcomeIn.String():
		return WorkflowConditionOutcomeIn
	case WorkflowConditionRuleHit.String():
		return WorkflowConditionRuleHit
	case WorkflowPayloadEvaluates.String():
		return WorkflowPayloadEvaluates
	default:
		return WorkflowConditionUnknown
	}
}

type WorkflowCondition struct {
	Id       uuid.UUID
	RuleId   uuid.UUID
	Function WorkflowConditionType
	Params   json.RawMessage

	CreatedAt time.Time
	UpdatedAt *time.Time
}

type WorkflowAction struct {
	Id     uuid.UUID
	RuleId uuid.UUID
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

type WorkflowExecution struct {
	AddedToCase bool
	WebhookIds  []string
}
