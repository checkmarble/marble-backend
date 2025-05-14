package evaluate_scenario

import (
	"context"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

const (
	ErrSanctionCheckAllFieldsNullOrEmpty = "all_fields_null_or_empty"
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

	start := time.Now()

	if iteration.SanctionCheckConfig.TriggerRule != nil {
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
	}

	queries := []models.OpenSanctionsCheckQuery{}
	emptyFieldRead := 0
	nbEvaluatedFields := 0
	emptyInput := false
	var err error

	if iteration.SanctionCheckConfig.Query.Name != nil {
		nbEvaluatedFields += 1
		queries, emptyInput, err = e.evaluateSanctionCheckName(ctx, queries, iteration, dataAccessor)
		if err != nil {
			return nil, true, errors.Wrap(err, "could not evaluate sanction check name")
		} else if emptyInput {
			emptyFieldRead += 1
		}
	}

	var nameRecognitionDuration time.Duration

	if iteration.SanctionCheckConfig.Query.Label != nil {
		nbEvaluatedFields += 1
		nameRecognizedQueries, emptyInput, duration, err :=
			e.evaluateSanctionCheckLabel(ctx, queries, iteration, dataAccessor)
		if err != nil {
			return nil, true, errors.Wrap(err, "could not evaluate sanction check label")
		} else if emptyInput {
			emptyFieldRead += 1
		}

		queries = nameRecognizedQueries
		nameRecognitionDuration = duration
	}

	if emptyFieldRead == nbEvaluatedFields {
		sanctionCheck = &models.SanctionCheckWithMatches{
			SanctionCheck: models.SanctionCheck{
				Status:     models.SanctionStatusError,
				ErrorCodes: []string{ErrSanctionCheckAllFieldsNullOrEmpty},
			},
		}

		performed = false
		return
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

		counterpartyId, ok := counterpartyIdResult.ReturnValue.(string)
		if counterpartyIdResult.ReturnValue == nil || !ok {
			sanctionCheckErr = errors.Wrapf(err, "could not parse object field for white list check as string, read %v", counterpartyIdResult.ReturnValue)
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
	sanctionCheck.Duration = time.Since(start)
	sanctionCheck.NameRecognitionDuration = nameRecognitionDuration
	performed = true
	return
}

func (e ScenarioEvaluator) evaluateSanctionCheckName(
	ctx context.Context,
	queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration,
	dataAccessor DataAccessor,
) (queriesOut []models.OpenSanctionsCheckQuery, emptyInput bool, err error) {
	queriesOut = queries
	nameFilterAny, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
		*iteration.SanctionCheckConfig.Query.Name, iteration.OrganizationId,
		dataAccessor.ClientObject, dataAccessor.DataModel)
	if err != nil {
		return
	}
	if nameFilterAny.ReturnValue == nil {
		emptyInput = true
		return
	}

	nameFilter, ok := nameFilterAny.ReturnValue.(string)
	if !ok {
		return nil, false, errors.New("name filter name query did not return a string")
	}
	if nameFilter == "" {
		emptyInput = true
		return
	}

	queriesOut = append(queriesOut, models.OpenSanctionsCheckQuery{
		Type: "Thing",
		Filters: models.OpenSanctionCheckFilter{
			"name": []string{nameFilter},
		},
	})

	return queriesOut, false, nil
}

func (e ScenarioEvaluator) evaluateSanctionCheckLabel(
	ctx context.Context,
	queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration,
	dataAccessor DataAccessor,
) (queriesOut []models.OpenSanctionsCheckQuery, emptyInput bool, took time.Duration, err error) {
	queriesOut = queries
	labelFilterAny, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
		*iteration.SanctionCheckConfig.Query.Label, iteration.OrganizationId,
		dataAccessor.ClientObject, dataAccessor.DataModel)
	if err != nil {
		return
	}
	if labelFilterAny.ReturnValue == nil {
		emptyInput = true
		return
	}

	labelFilter, ok := labelFilterAny.ReturnValue.(string)
	if !ok {
		return nil, false, 0, errors.New("label filter name query did not return a string")
	}
	if labelFilter == "" {
		emptyInput = true
		return
	}

	if e.nameRecognizer == nil || !e.nameRecognizer.IsConfigured() {
		switch len(queriesOut) {
		case 0:
			queriesOut = append(queriesOut, models.OpenSanctionsCheckQuery{
				Type: "Thing",
				Filters: models.OpenSanctionCheckFilter{
					"name": []string{labelFilter},
				},
			})

		default:
			queriesOut[0].Filters["name"] = append(queriesOut[0].Filters["name"], labelFilter)
		}

		return queriesOut, false, 0, nil
	}

	beforeNameRecognition := time.Now()

	matches, err := e.nameRecognizer.PerformNameRecognition(ctx, labelFilter)
	if err != nil {
		return queriesOut, false, 0, errors.Wrap(err,
			"could not perform name recognition on label")
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
		queriesOut = append(queriesOut, *personQuery)
	}
	if companyQuery != nil {
		queriesOut = append(queriesOut, *companyQuery)
	}

	return queriesOut, false, time.Since(beforeNameRecognition), nil
}
