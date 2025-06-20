package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type WorkflowRuleDto struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type PostWorkflowRuleDto struct {
	Name string `json:"name" binding:"required"`
}

type WorkflowConditionDto struct {
	Id       string          `json:"id"`
	Function string          `json:"function"`
	Params   json.RawMessage `json:"params"`
}

type WorkflowActionDto struct {
	Id     string          `json:"id"`
	Action string          `json:"action"`
	Params json.RawMessage `json:"params"`
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
		Id:   m.Id,
		Name: m.Name,
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
