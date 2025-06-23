package decision_workflows

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type DecisionWorkflowsCondition func(context.Context, DecisionWorkflowRequest) bool

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

		return ifOutcomeIn(params), nil
	case models.WorkflowConditionRuleHit:
		var params dto.WorkflowConditionRuleHitParams

		if err := json.Unmarshal(condition.Params, &params); err != nil {
			return never, err
		}

		return ifRuleHit(params.RuleId), nil
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

func always(ctx context.Context, req DecisionWorkflowRequest) bool {
	return true
}

func never(ctx context.Context, req DecisionWorkflowRequest) bool {
	return false
}

func ifOutcomeIn(outcomes []string) DecisionWorkflowsCondition {
	return func(ctx context.Context, req DecisionWorkflowRequest) bool {
		return slices.Contains(outcomes, req.Decision.Outcome.String())
	}
}

func ifRuleHit(ruleId string) DecisionWorkflowsCondition {
	return func(ctx context.Context, req DecisionWorkflowRequest) bool {
		for _, ruleExec := range req.Decision.RuleExecutions {
			if ruleExec.Rule.StableRuleId != nil && *ruleExec.Rule.StableRuleId == ruleId && ruleExec.Outcome == "hit" {
				return true
			}
		}

		return false
	}
}

func payloadEvaluates(astNode ast.Node) DecisionWorkflowsCondition {
	return func(ctx context.Context, req DecisionWorkflowRequest) bool {
		eval, err := req.EvaluateAst.EvaluateAstExpression(ctx, nil, astNode, req.Scenario.OrganizationId, req.Params.ClientObject, req.Params.DataModel)
		if err != nil {
			return false
		}
		if len(eval.Errors) > 0 {
			return false
		}
		ret, ok := eval.ReturnValue.(bool)
		if !ok {
			return false
		}

		return ret
	}
}
