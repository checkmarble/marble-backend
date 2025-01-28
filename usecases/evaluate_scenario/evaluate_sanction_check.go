package evaluate_scenario

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

func evaluateSanctionCheck(ctx context.Context, repositories ScenarioEvaluationRepositories,
	iteration models.ScenarioIteration, params ScenarioEvaluationParameters, dataAccessor DataAccessor,
) (sanctionCheck *models.SanctionCheck, performed bool, sanctionCheckErr error) {
	if iteration.SanctionCheckConfig != nil && iteration.SanctionCheckConfig.Enabled {
		triggerEvaluation, err := repositories.EvaluateAstExpression.EvaluateAstExpression(
			ctx,
			*iteration.SanctionCheckConfig.TriggerRule,
			params.Scenario.OrganizationId,
			dataAccessor.ClientObject,
			params.DataModel,
		)
		if err != nil {
			sanctionCheckErr = errors.Wrap(err, "could not execute sanction check trigger rule")
			return
		}
		if _, ok := triggerEvaluation.ReturnValue.(bool); !ok {
			sanctionCheckErr = errors.New("sanction check trigger rule did not evaluate to a boolean")
		}

		if triggerEvaluation.ReturnValue == true {
			query := models.OpenSanctionsQuery{
				Config: *iteration.SanctionCheckConfig,
				Queries: models.OpenSanctionCheckFilter{
					// TODO: take this from the context and the scenario configuration
					"name": []string{"obama"},
				},
			}

			result, err := repositories.EvalSanctionCheckUsecase.Execute(ctx,
				params.Scenario.OrganizationId, *iteration.SanctionCheckConfig, query)
			if err != nil {
				sanctionCheckErr = errors.Wrap(err, "could not perform sanction check")
				return
			}

			sanctionCheck = &result
			performed = true
		}
	}

	return
}
