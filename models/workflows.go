package models

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type WorkflowActionType string

const (
	// generic
	WorkflowDisabled WorkflowActionType = "DISABLED"

	// decision workflows
	WorkflowCreateCase          WorkflowActionType = "CREATE_CASE"
	WorkflowAddToCaseIfPossible WorkflowActionType = "ADD_TO_CASE_IF_POSSIBLE"

	// case workflows
	WorkflowGenerateAiCaseReview WorkflowActionType = "GENERATE_AI_CASE_REVIEW"
)

func WorkflowActionTypeFromString(s string) WorkflowActionType {
	switch s {
	case "ADD_TO_CASE_IF_POSSIBLE":
		return WorkflowAddToCaseIfPossible
	case "CREATE_CASE":
		return WorkflowCreateCase
	case "GENERATE_AI_CASE_REVIEW":
		return WorkflowGenerateAiCaseReview
	default:
		return WorkflowDisabled
	}
}

type WorkflowRuleType int

const (
	WorkflowRuleTypeUnknown WorkflowRuleType = iota
	WorkflowRuleTypeDecision
	WorkflowRuleTypeCase
)

func (t WorkflowRuleType) String() string {
	switch t {
	case WorkflowRuleTypeDecision:
		return "decision"
	case WorkflowRuleTypeCase:
		return "case"
	default:
		return "unknown"
	}
}

func WorkflowRuleTypeFromString(s string) WorkflowRuleType {
	switch s {
	case "decision":
		return WorkflowRuleTypeDecision
	case "case":
		return WorkflowRuleTypeCase
	default:
		return WorkflowRuleTypeUnknown
	}
}

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
	Type        WorkflowRuleType

	CreatedAt time.Time
	UpdatedAt *time.Time
}

type WorkflowConditionType string

const (
	// generic
	WorkflowConditionUnknown WorkflowConditionType = "unknown"

	// scenario (decision) workflows
	WorkflowConditionAlways    WorkflowConditionType = "always"
	WorkflowConditionNever     WorkflowConditionType = "never"
	WorkflowConditionOutcomeIn WorkflowConditionType = "outcome_in"
	WorkflowConditionRuleHit   WorkflowConditionType = "rule_hit"
	WorkflowPayloadEvaluates   WorkflowConditionType = "payload_evaluates"

	// case workflows
	WorkflowConditionCaseCreated   WorkflowConditionType = "case_created"
	WorkflowConditionCaseExcalated WorkflowConditionType = "case_excalated"
)

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
	case WorkflowConditionCaseCreated.String():
		return WorkflowConditionCaseCreated
	case WorkflowConditionCaseExcalated.String():
		return WorkflowConditionCaseExcalated
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
	Action WorkflowActionType
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
	Action WorkflowActionType
	Params T
}

type WorkflowExecution struct {
	AddedToCase bool
	WebhookIds  []string
}
