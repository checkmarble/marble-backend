package decision_workflows

import (
	"context"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/utils"
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
	logger := utils.LoggerFromContext(ctx)

	req := DecisionWorkflowRequest{
		Scenario:    scenario,
		Decision:    decision,
		Params:      evalParams,
		EvaluateAst: d.astEvaluator,
	}

	var matchingRules []models.Workflow

Rule:
	for _, rule := range rules {
		if len(rule.Conditions) == 0 {
			continue
		}

		for _, cond := range rule.Conditions {
			fn, err := CreateFunction(cond)
			if err != nil {
				return models.WorkflowExecution{}, errors.Wrap(err, "could not evaluate workflow condition")
			}

			result, err := fn(ctx, req)
			if err != nil {
				logger.Warn("error while executing workflow condition",
					"decision", decision.Decision,
					"condition", cond.Id,
					"error", err.Error())

				return models.WorkflowExecution{}, err
			}

			if !result {
				continue Rule
			}
		}

		matchingRules = append(matchingRules, rule)

		if !rule.Fallthrough {
			break
		}
	}

	performed := models.WorkflowExecution{
		WebhookIds: make([]string, 0),
	}

	for _, rule := range matchingRules {
		for _, action := range rule.Actions {
			switch action.Action {
			case models.WorkflowCreateCase, models.WorkflowAddToCaseIfPossible:
				// Skip if a previous executed action already added the decision to a case.
				if !performed.AddedToCase {
					params, err := models.ParseWorkflowAction[dto.WorkflowActionCaseParams](action)
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
	}

	return performed, nil
}
