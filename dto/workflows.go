package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

type WorkflowRuleDto struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type CreateWorkflowRuleDto struct {
	ScenarioId string `json:"scenario_id" binding:"required,uuid"`
	Name       string `json:"name" binding:"required"`
}

type UpdateWorkflowRuleDto struct {
	Name string `json:"name" binding:"required"`
}

type WorkflowConditionDto struct {
	Id       string                       `json:"id"`
	Function models.WorkflowConditionType `json:"function"`
	Params   json.RawMessage              `json:"params,omitempty"`
}

type PostWorkflowConditionDto struct {
	Function models.WorkflowConditionType `json:"function" binding:"required"`
	Params   json.RawMessage              `json:"params"`
}

type WorkflowConditionRuleHitParams struct {
	RuleId string `json:"rule_id" binding:"required,uuid"`
}

type WorkflowConditionScreeningHitParams struct {
	ScreeningId string `json:"screening_id" binding:"required,uuid"`
}

type WorkflowConditionEvaluatesParams struct {
	Expression NodeDto `json:"expression" binding:"required"`
}

type WorkflowActionDto struct {
	Id     string          `json:"id"`
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
}

type PostWorkflowActionDto struct {
	Action models.WorkflowType `json:"action" binding:"required"`
	Params json.RawMessage     `json:"params"`
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

func ValidateWorkflowAction(cond PostWorkflowActionDto) error {
	switch cond.Action {
	case models.WorkflowCreateCase, models.WorkflowAddToCaseIfPossible:
		var params models.WorkflowCaseParams

		if err := json.Unmarshal(cond.Params, &params); err != nil {
			return errors.Join(models.BadParameterError, json.Unmarshal(cond.Params, new(models.WorkflowCaseParams)))
		}
		if err := binding.Validator.Engine().(*validator.Validate).Struct(params); err != nil {
			return errors.Join(models.BadParameterError, err)
		}
	default:
		return errors.Wrapf(models.BadParameterError, "unknown workflow action type: %s", cond.Action)
	}

	return nil
}
