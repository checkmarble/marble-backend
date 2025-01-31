package evaluate_scenario

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/cockroachdb/errors"
)

func evaluateSanctionCheck(ctx context.Context,
	evaluator ast_eval.EvaluateAstExpression, executor EvalSanctionCheckUsecase, nameRecognizer EvalNameRecognitionRepository,
	iteration models.ScenarioIteration, params ScenarioEvaluationParameters, dataAccessor DataAccessor,
) (sanctionCheck *models.SanctionCheckWithMatches, performed bool, sanctionCheckErr error) {
	if iteration.SanctionCheckConfig != nil && iteration.SanctionCheckConfig.Enabled {
		triggerEvaluation, err := evaluator.EvaluateAstExpression(
			ctx,
			iteration.SanctionCheckConfig.TriggerRule,
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
			mainQuery := models.OpenSanctionCheckQuery{
				Type: "Thing",
				Filters: models.OpenSanctionCheckFilter{
					"name": {},
				},
			}

			queries := []models.OpenSanctionCheckQuery{mainQuery}

			if nameRecognizer != nil && iteration.SanctionCheckConfig.Query.Label != nil {
				queries, err = evaluateSanctionCheckLabel(ctx, queries, evaluator,
					nameRecognizer, iteration, dataAccessor)
				if err != nil {
					return nil, true, err
				}
			}

			if err := evaluateSanctionCheckName(ctx, &mainQuery, evaluator, iteration, dataAccessor); err != nil {
				return nil, true, err
			}

			query := models.OpenSanctionsQuery{
				Config:  *iteration.SanctionCheckConfig,
				Queries: queries,
			}

			result, err := executor.Execute(ctx,
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

func evaluateSanctionCheckName(ctx context.Context, query *models.OpenSanctionCheckQuery,
	evaluator ast_eval.EvaluateAstExpression, iteration models.ScenarioIteration, dataAccessor DataAccessor,
) error {
	nameFilterAny, err := evaluator.EvaluateAstExpression(ctx,
		iteration.SanctionCheckConfig.Query.Name, iteration.OrganizationId,
		dataAccessor.ClientObject, dataAccessor.DataModel)
	if err != nil {
		return err
	}

	nameFilter, ok := nameFilterAny.ReturnValue.(string)
	if !ok {
		return errors.New("name filter name query did not return a string")
	}

	query.Filters["name"] = append(query.Filters["name"], nameFilter)

	return nil
}

func evaluateSanctionCheckLabel(ctx context.Context, queries []models.OpenSanctionCheckQuery,
	evaluator ast_eval.EvaluateAstExpression, nameRecognizer EvalNameRecognitionRepository,
	iteration models.ScenarioIteration, dataAccessor DataAccessor,
) ([]models.OpenSanctionCheckQuery, error) {
	labelFilterAny, err := evaluator.EvaluateAstExpression(ctx,
		*iteration.SanctionCheckConfig.Query.Label, iteration.OrganizationId,
		dataAccessor.ClientObject, dataAccessor.DataModel)
	if err != nil {
		return queries, err
	}

	labelFilter, ok := labelFilterAny.ReturnValue.(string)
	if !ok {
		return queries, errors.New("label filter name query did not return a string")
	}

	matches, err := nameRecognizer.Detect(ctx, labelFilter)
	if err != nil {
		return queries, errors.New("could not perform name recognition on label")
	}

	for _, match := range matches {
		switch match.Type {
		case "Person":
			queries[0].Filters["name"] = append(queries[0].Filters["name"], match.Text)
		case "Company":
			queries = append(queries, models.OpenSanctionCheckQuery{
				Type: "Company",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{match.Text},
				},
			})
		}
	}

	return queries, nil
}
