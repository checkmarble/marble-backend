package decision_workflows

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/checkmarble/marble-backend/models"
)

type DecisionWorkflowsCondition func(context.Context, DecisionWorkflowRequest) bool

func CreateFunction(condition models.WorkflowCondition) (DecisionWorkflowsCondition, error) {
	switch condition.Function {
	case "always":
		return always, nil
	case "never":
		return never, nil
	case "if_outcome_in":
		var params []string

		if err := json.Unmarshal(condition.Params, &params); err != nil {
			return never, err
		}

		return ifOutcomeIn(params), nil
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
