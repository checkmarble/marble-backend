package evaluate_scenario

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
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
		*iteration.SanctionCheckConfig.TriggerRule,
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

	mainQuery := models.OpenSanctionsCheckQuery{
		Type: "Thing",
		Filters: models.OpenSanctionCheckFilter{
			"name": {},
		},
	}

	queries := []models.OpenSanctionsCheckQuery{mainQuery}

	if e.nameRecognizer != nil && iteration.SanctionCheckConfig.Query.Label != nil {
		queries, err = e.evaluateSanctionCheckLabel(ctx, queries, iteration, dataAccessor)
		if err != nil {
			return nil, true, err
		}
	}

	// Then, actually perform the sanction check
	if err := e.evaluateSanctionCheckName(ctx, &mainQuery, iteration, dataAccessor); err != nil {
		return nil, true, err
	}

	var uniqueCounterpartyIdentifier *string

	query := models.OpenSanctionsQuery{
		Config:  *iteration.SanctionCheckConfig,
		Queries: queries,
	}

	if iteration.SanctionCheckConfig.CounterpartyIdExpression != nil {
		counterpartyIdResult, err := e.evaluateAstExpression.EvaluateAstExpression(
			ctx,
			nil,
			*iteration.SanctionCheckConfig.CounterpartyIdExpression,
			params.Scenario.OrganizationId,
			dataAccessor.ClientObject,
			params.DataModel,
		)
		if err != nil {
			sanctionCheckErr = errors.Wrap(err, "could not extract object field for whitelist check")
			return
		}

		counterpartyId, err := counterpartyIdResult.GetStringReturnValue()
		if err != nil && !errors.Is(err, ast.ErrNullFieldRead) {
			sanctionCheckErr = errors.Wrap(err, "could not parse object field for white list check as string")
			return
		}

		if trimmed := strings.TrimSpace(counterpartyId); trimmed != "" {
			uniqueCounterpartyIdentifier = &counterpartyId

			whitelistCount, err := e.evalSanctionCheckUsecase.CountWhitelistsForCounterpartyId(
				ctx, iteration.OrganizationId, *uniqueCounterpartyIdentifier)
			if err != nil {
				sanctionCheckErr = errors.Wrap(err, "could not retrieve whitelist count")
				return
			}

			query.LimitIncrease = whitelistCount
		}
	}

	result, err := e.evalSanctionCheckUsecase.Execute(ctx, params.Scenario.OrganizationId, query)
	if err != nil {
		sanctionCheckErr = errors.Wrap(err, "could not perform sanction check")
		return
	}

	if uniqueCounterpartyIdentifier != nil {
		for idx := range result.Matches {
			result.Matches[idx].UniqueCounterpartyIdentifier = uniqueCounterpartyIdentifier
		}

		result, err = e.evalSanctionCheckUsecase.FilterOutWhitelistedMatches(ctx,
			params.Scenario.OrganizationId, result, *uniqueCounterpartyIdentifier)
		if err != nil {
			return
		}
	}

	sanctionCheck = &result
	performed = true
	return
}

func (e ScenarioEvaluator) evaluateSanctionCheckName(ctx context.Context, query *models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration, dataAccessor DataAccessor,
) error {
	nameFilterAny, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
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

func (e ScenarioEvaluator) evaluateSanctionCheckLabel(ctx context.Context, queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration, dataAccessor DataAccessor,
) ([]models.OpenSanctionsCheckQuery, error) {
	labelFilterAny, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
		*iteration.SanctionCheckConfig.Query.Label, iteration.OrganizationId,
		dataAccessor.ClientObject, dataAccessor.DataModel)
	if err != nil {
		return queries, err
	}

	labelFilter, ok := labelFilterAny.ReturnValue.(string)
	if !ok {
		return queries, errors.New("label filter name query did not return a string")
	}

	matches, err := e.nameRecognizer.PerformNameRecognition(ctx, labelFilter)
	if err != nil {
		return queries, errors.New("could not perform name recognition on label")
	}

	var personQuery *models.OpenSanctionsCheckQuery = nil
	var companyQuery *models.OpenSanctionsCheckQuery = nil

	for _, match := range matches {
		switch match.Type {
		case "Person":
			if personQuery == nil {
				personQuery = &models.OpenSanctionsCheckQuery{
					Type:    "Person",
					Filters: models.OpenSanctionCheckFilter{"name": []string{match.Text}},
				}
				continue
			}

			personQuery.Filters["name"] = append(personQuery.Filters["name"], match.Text)
		case "Company":
			if companyQuery == nil {
				companyQuery = &models.OpenSanctionsCheckQuery{
					Type:    "Organization",
					Filters: models.OpenSanctionCheckFilter{"name": []string{match.Text}},
				}
				continue
			}

			companyQuery.Filters["name"] = append(companyQuery.Filters["name"], match.Text)
		}
	}

	if personQuery != nil {
		queries = append(queries, *personQuery)
	}
	if companyQuery != nil {
		queries = append(queries, *companyQuery)
	}

	return queries, nil
}
