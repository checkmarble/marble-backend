package decision_workflows

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/pkg/errors"
)

type DecisionWorkflowRule struct {
	Conditions []DecisionWorkflowsCondition `json:"conditions"`
	Action     models.WorkflowType          `json:"action"`
}

type DecisionWorkflowRequest struct {
	Scenario    models.Scenario
	Decision    models.DecisionWithRuleExecutions
	Params      evaluate_scenario.ScenarioEvaluationParameters
	EvaluateAst ast_eval.EvaluateAstExpression
}

func (d DecisionsWorkflows) ProcessDecisionWorkflows(
	ctx context.Context,
	tx repositories.Transaction,
	rules []models.Workflow,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	evalParams evaluate_scenario.ScenarioEvaluationParameters,
) (models.WorkflowExecution, error) {
	req := DecisionWorkflowRequest{
		Scenario:    scenario,
		Decision:    decision,
		Params:      evalParams,
		EvaluateAst: d.astEvaluator,
	}

	var matchingRule *models.Workflow

Rule:
	for _, rule := range rules {
		for _, cond := range rule.Conditions {
			fn, err := CreateFunction(cond)
			if err != nil {
				return models.WorkflowExecution{}, errors.Wrap(err, "could not evaluate workflow condition")
			}

			if !fn(ctx, req) {
				continue Rule
			}
		}

		matchingRule = &rule
		break
	}

	performed := models.WorkflowExecution{
		WebhookIds: make([]string, 0),
	}

	if matchingRule != nil {
		for _, action := range matchingRule.Actions {
			switch action.Action {
			case models.WorkflowCreateCase, models.WorkflowAddToCaseIfPossible:
				params, err := models.ParseWorkflowAction[models.WorkflowCaseParams](action)
				if err != nil {
					return models.WorkflowExecution{}, errors.Wrap(err, "could not unmarshal workflow action parameters")
				}

				exec, err := d.AutomaticDecisionToCase(ctx, tx, scenario, decision, evalParams, params)
				if err != nil {
					return models.WorkflowExecution{}, errors.Wrap(err, "error while executing workflow action")
				}

				performed.AddedToCase = performed.AddedToCase || exec.AddedToCase
				performed.WebhookIds = append(performed.WebhookIds, exec.WebhookIds...)
			}
		}
	}

	return performed, nil
}
