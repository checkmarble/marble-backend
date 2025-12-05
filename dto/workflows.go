package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type WorkflowRuleDto struct {
	Id          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Fallthrough bool      `json:"fallthrough"`
}

type CreateWorkflowRuleDto struct {
	ScenarioId  uuid.UUID `json:"scenario_id" binding:"required,uuid"`
	Name        string    `json:"name" binding:"required"`
	Fallthrough *bool     `json:"fallthrough" binding:"required"`
}

type UpdateWorkflowRuleDto struct {
	Name        string `json:"name" binding:"required"`
	Fallthrough *bool  `json:"fallthrough" binding:"required"`
}

type WorkflowConditionDto struct {
	Id       uuid.UUID                    `json:"id"`
	Function models.WorkflowConditionType `json:"function"`
	Params   json.RawMessage              `json:"params,omitempty"`
}

type PostWorkflowConditionDto struct {
	Function models.WorkflowConditionType `json:"function" binding:"required"`
	Params   json.RawMessage              `json:"params"`
}

type WorkflowConditionRuleHitParams struct {
	RuleId []uuid.UUID `json:"rule_id" binding:"required"`
}

type WorkflowConditionEvaluatesParams struct {
	Expression NodeDto `json:"expression" binding:"required"`
}

type WorkflowActionDto struct {
	Id     uuid.UUID       `json:"id"`
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
}

type PostWorkflowActionDto struct {
	Action models.WorkflowType `json:"action" binding:"required"`
	Params json.RawMessage     `json:"params"`
}

type WorkflowActionCaseParams struct {
	InboxId       uuid.UUID   `json:"inbox_id" binding:"required,uuid"`
	AnyInbox      bool        `json:"any_inbox"`
	TitleTemplate *NodeDto    `json:"title_template"`
	TagsToAdd     []uuid.UUID `json:"tags_to_add"`
}

type WorkflowDto struct {
	WorkflowRuleDto

	Conditions []WorkflowConditionDto `json:"conditions"`
	Actions    []WorkflowActionDto    `json:"actions"`
}

func AdaptWorkflow(m models.Workflow) WorkflowDto {
	return WorkflowDto{
		WorkflowRuleDto: AdaptWorkflowRule(m.WorkflowRule),
		Conditions:      pure_utils.Map(m.Conditions, AdaptWorkflowCondition),
		Actions:         pure_utils.Map(m.Actions, AdaptWorkflowAction),
	}
}

func AdaptWorkflowRule(m models.WorkflowRule) WorkflowRuleDto {
	return WorkflowRuleDto{
		Id:          m.Id,
		Name:        m.Name,
		Fallthrough: m.Fallthrough,
	}
}

func AdaptWorkflowCondition(m models.WorkflowCondition) WorkflowConditionDto {
	return WorkflowConditionDto{
		Id:       m.Id,
		Function: m.Function,
		Params:   m.Params,
	}
}

func AdaptWorkflowAction(m models.WorkflowAction) WorkflowActionDto {
	return WorkflowActionDto{
		Id:     m.Id,
		Action: string(m.Action),
		Params: m.Params,
	}
}
