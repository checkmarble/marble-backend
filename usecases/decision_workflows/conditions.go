package decision_workflows

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
)

type DecisionWorkflowsCondition func(context.Context, DecisionWorkflowRequest) (bool, error)

func CreateFunction(condition models.WorkflowCondition) (DecisionWorkflowsCondition, error) {
	switch condition.Function {
	case models.WorkflowConditionAlways:
		return always, nil
	case models.WorkflowConditionNever:
		return never, nil
	case models.WorkflowConditionOutcomeIn:
		var params []string

		if err := json.Unmarshal(condition.Params, &params); err != nil {
			return never, err
		}

		return outcomeIn(params), nil
	case models.WorkflowConditionRuleHit:
		var params dto.WorkflowConditionRuleHitParams

		if err := json.Unmarshal(condition.Params, &params); err != nil {
			return never, err
		}

		return ruleHit(params.RuleId), nil
	case models.WorkflowPayloadEvaluates:
		var params dto.WorkflowConditionEvaluatesParams

		if err := json.Unmarshal(condition.Params, &params); err != nil {
			return never, err
		}

		astNode, err := dto.AdaptASTNode(params.Expression)
		if err != nil {
			return never, err
		}

		return payloadEvaluates(astNode), nil
	default:
		return never, nil
	}
}

func always(ctx context.Context, req DecisionWorkflowRequest) (bool, error) {
	return true, nil
}

func never(ctx context.Context, req DecisionWorkflowRequest) (bool, error) {
	return false, nil
}

func outcomeIn(outcomes []string) DecisionWorkflowsCondition {
	return func(ctx context.Context, req DecisionWorkflowRequest) (bool, error) {
		return slices.Contains(outcomes, req.Decision.Outcome.String()), nil
	}
}

func ruleHit(ruleIds []uuid.UUID) DecisionWorkflowsCondition {
	return func(ctx context.Context, req DecisionWorkflowRequest) (bool, error) {
		for _, ruleExec := range req.Decision.RuleExecutions {
			for _, ruleId := range ruleIds {
				if ruleExec.Rule.StableRuleId == ruleId.String() && ruleExec.Outcome == "hit" {
					return true, nil
				}
			}
		}
		for _, screeningExec := range req.Decision.ScreeningExecutions {
			for _, ruleId := range ruleIds {
				if screeningExec.Config.StableId == ruleId.String() && (screeningExec.Status == models.ScreeningStatusInReview || screeningExec.Status == models.ScreeningStatusConfirmedHit) {
					return true, nil
				}
			}
		}

		return false, nil
	}
}

func payloadEvaluates(astNode ast.Node) DecisionWorkflowsCondition {
	return func(ctx context.Context, req DecisionWorkflowRequest) (bool, error) {
		eval, err := req.EvaluateAst.EvaluateAstExpression(ctx, nil, astNode, req.Scenario.OrganizationId, req.Params.ClientObject, req.Params.DataModel)
		if err != nil {
			return false, err
		}
		if len(eval.Errors) > 0 {
			return false, nil
		}
		ret, ok := eval.ReturnValue.(bool)
		if !ok {
			return false, ast.ErrArgumentMustBeBool
		}

		return ret, nil
	}
}
