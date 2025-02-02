package evaluate_scenario

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/cockroachdb/errors"
)

func evaluateSanctionCheck(
	ctx context.Context,
	evaluator ast_eval.EvaluateAstExpression,
	executor EvalSanctionCheckUsecase,
	iteration models.ScenarioIteration,
	params ScenarioEvaluationParameters,
	dataAccessor DataAccessor,
) (
	sanctionCheck *models.SanctionCheckWithMatches,
	performed bool,
	sanctionCheckErr error,
) {
	// First, check if the sanction check should be performed
	if iteration.SanctionCheckConfig == nil || !iteration.SanctionCheckConfig.Enabled {
		return
	}

	triggerEvaluation, err := evaluator.EvaluateAstExpression(
		ctx,
		nil,
		iteration.SanctionCheckConfig.TriggerRule,
		params.Scenario.OrganizationId,
		dataAccessor.ClientObject,
		params.DataModel,
	)
	if err != nil {
		sanctionCheckErr = errors.Wrap(err, "could not execute sanction check trigger rule")
		return
	}
	passed, ok := triggerEvaluation.ReturnValue.(bool)
	if !ok {
		sanctionCheckErr = errors.New("sanction check trigger rule did not evaluate to a boolean")
	} else if !passed {
		return
	}

	// Then, actually perform the sanction check
	nameFilterAny, err := evaluator.EvaluateAstExpression(
		ctx,
		nil,
		iteration.SanctionCheckConfig.Query.Name,
		iteration.OrganizationId,
		dataAccessor.ClientObject,
		dataAccessor.DataModel)
	if err != nil {
		return nil, true, err
	}
	nameFilter, ok := nameFilterAny.ReturnValue.(string)
	if !ok {
		return nil, true, errors.New("name filter name query did not return a string")
	}

	query := models.OpenSanctionsQuery{
		Config: *iteration.SanctionCheckConfig,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{nameFilter},
		},
	}

	result, err := executor.Execute(ctx, params.Scenario.OrganizationId, query)
	if err != nil {
		sanctionCheckErr = errors.Wrap(err, "could not perform sanction check")
		return
	}

	sanctionCheck = &result
	performed = true
	return
}
