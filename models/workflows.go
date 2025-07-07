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
	WorkflowConditionAlways                          = "always"
	WorkflowConditionNever                           = "never"
	WorkflowConditionOutcomeIn                       = "outcome_in"
	WorkflowConditionRuleHit                         = "rule_hit"
	WorkflowPayloadEvaluates                         = "payload_evaluates"
)

var (
	ValidWorkflowConditions = []WorkflowConditionType{
		WorkflowConditionAlways,
		WorkflowConditionNever,
		WorkflowConditionOutcomeIn,
		WorkflowConditionRuleHit,
	}
)

func WorkflowConditionFromString(s string) WorkflowConditionType {
	switch s {
	case WorkflowConditionAlways:
		return WorkflowConditionAlways
	case WorkflowConditionNever:
		return WorkflowConditionNever
	case WorkflowConditionOutcomeIn:
		return WorkflowConditionOutcomeIn
	case WorkflowConditionRuleHit:
		return WorkflowConditionRuleHit
	case WorkflowPayloadEvaluates:
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
