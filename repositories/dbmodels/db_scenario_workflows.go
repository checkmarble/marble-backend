package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

type DbWorkflowRule struct {
	Id         string `db:"id"`
	ScenarioId string `db:"scenario_id"`
	Name       string `db:"name"`
	Priority   int    `db:"priority"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

type DbWorkflowRuleWithConditions struct {
	DbWorkflowRule

	Conditions []DbWorkflowCondition `db:"conditions"`
	Actions    []DbWorkflowAction    `db:"actions"`
}

type DbWorkflowCondition struct {
	Id       string          `db:"id"`
	RuleId   string          `db:"rule_id"`
	Function string          `db:"function"`
	Params   json.RawMessage `db:"params"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

type DbWorkflowAction struct {
	Id     string          `db:"id"`
	RuleId string          `db:"rule_id"`
	Action string          `db:"action"`
	Params json.RawMessage `db:"params"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

const TABLE_WORKFLOW_RULES = "scenario_workflow_rules"
const TABLE_WORKFLOW_CONDITIONS = "scenario_workflow_conditions"
const TABLE_WORKFLOW_ACTIONS = "scenario_workflow_actions"

var WorkflowRuleColumns = utils.ColumnList[DbWorkflowRule]()
var WorkflowConditionColumns = utils.ColumnList[DbWorkflowCondition]()
var WorkflowActionColumns = utils.ColumnList[DbWorkflowAction]()

func AdaptWorkflowRule(db DbWorkflowRule) (models.WorkflowRule, error) {
	return models.WorkflowRule{
		Id:         db.Id,
		ScenarioId: db.ScenarioId,
		Name:       db.Name,
		Priority:   db.Priority,
		CreatedAt:  db.CreatedAt,
		UpdatedAt:  db.UpdatedAt,
	}, nil
}

func AdaptWorkflowCondition(db DbWorkflowCondition) (models.WorkflowCondition, error) {
	return models.WorkflowCondition{
		Id:        db.Id,
		RuleId:    db.RuleId,
		Function:  db.Function,
		Params:    db.Params,
		CreatedAt: db.CreatedAt,
		UpdatedAt: db.UpdatedAt,
	}, nil
}

func AdaptWorkflowAction(db DbWorkflowAction) (models.WorkflowAction, error) {
	return models.WorkflowAction{
		Id:        db.Id,
		RuleId:    db.RuleId,
		Action:    models.WorkflowTypeFromString(db.Action),
		Params:    db.Params,
		CreatedAt: db.CreatedAt,
		UpdatedAt: db.UpdatedAt,
	}, nil
}

func AdaptWorkflowRuleWithConditions(db DbWorkflowRuleWithConditions) (models.Workflow, error) {
	rule, err := AdaptWorkflowRule(db.DbWorkflowRule)
	if err != nil {
		return models.Workflow{}, err
	}

	conditions, err := pure_utils.MapErr(db.Conditions, AdaptWorkflowCondition)
	if err != nil {
		return models.Workflow{}, err
	}

	actions, err := pure_utils.MapErr(db.Actions, AdaptWorkflowAction)
	if err != nil {
		return models.Workflow{}, err
	}

	return models.Workflow{
		WorkflowRule: rule,
		Conditions:   conditions,
		Actions:      actions,
	}, nil
}
