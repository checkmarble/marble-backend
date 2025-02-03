package evaluate_scenario

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

func (e ScenarioEvaluator) evaluateSanctionCheck(
	ctx context.Context,
	iteration models.ScenarioIteration,
	params ScenarioEvaluationParameters,
	dataAccessor DataAccessor,
) (
	sanctionCheck *models.SanctionCheckWithMatches,
	performed bool,
	sanctionCheckErr error,
) {
	// First, check if the sanction check should be performed
	if iteration.SanctionCheckConfig == nil {
		return
	}

	triggerEvaluation, err := e.evaluateAstExpression.EvaluateAstExpression(
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
	nameFilterAny, err := e.evaluateAstExpression.EvaluateAstExpression(
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

	result, err := e.evalSanctionCheckUsecase.Execute(ctx, params.Scenario.OrganizationId, query)
	if err != nil {
		sanctionCheckErr = errors.Wrap(err, "could not perform sanction check")
		return
	}

	sanctionCheck = &result
	performed = true
	return
}
