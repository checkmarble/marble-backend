package evaluate_scenario

import (
	"context"
	"strings"
	"time"
	"unicode"

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
	sanctionCheck []models.SanctionCheckWithMatches,
	performed bool,
	sanctionCheckErr error,
) {
	// First, check if the sanction check should be performed
	if len(iteration.SanctionCheckConfigs) == 0 {
		return
	}

	for _, scc := range iteration.SanctionCheckConfigs {
		start := time.Now()

		if scc.TriggerRule != nil {
			triggerEvaluation, err := e.evaluateAstExpression.EvaluateAstExpression(
				ctx,
				nil,
				*scc.TriggerRule,
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
		var err error

		if scc.Query != nil {
			emptyInput := false

			switch scc.Preprocessing.UseNer {
			case false:
				queries, emptyInput, err = e.evaluateSanctionCheckName(ctx, queries, iteration, scc, dataAccessor)
			case true:
				queries, emptyInput, err = e.evaluateSanctionCheckLabel(ctx, queries, iteration, scc, dataAccessor)
			}

			if err != nil {
				return nil, true, errors.Wrap(err, "could not evaluate sanction check name")
			}

			// TODO: should we ignore a sanction check config that resolve to an empty counterparty name or fail?
			if emptyInput {
				sanctionCheck = append(sanctionCheck, models.SanctionCheckWithMatches{
					SanctionCheck: models.SanctionCheck{
						SanctionCheckConfigId: scc.Id,
						Status:                models.SanctionStatusError,
						ErrorCodes:            []string{ErrSanctionCheckAllFieldsNullOrEmpty},
					},
				})
				continue
			}
		}

		// TODO: what should we do when we do not resolve to any query?
		if len(queries) == 0 {
			sanctionCheck = append(sanctionCheck, models.SanctionCheckWithMatches{
				SanctionCheck: models.SanctionCheck{
					SanctionCheckConfigId: scc.Id,
					Status:                models.SanctionStatusNoHit,
				},
			})
			continue
		}

		var uniqueCounterpartyIdentifier *string

		query := models.OpenSanctionsQuery{
			Config:  scc,
			Queries: queries,
		}

		if scc.CounterpartyIdExpression != nil {
			counterpartyIdResult, err := e.evaluateAstExpression.EvaluateAstExpression(
				ctx,
				nil,
				*scc.CounterpartyIdExpression,
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

		result.SanctionCheckConfigId = scc.Id
		result.Duration = time.Since(start)

		sanctionCheck = append(sanctionCheck, result)
		performed = true
	}
	return
}

func (e ScenarioEvaluator) evaluateSanctionCheckName(
	ctx context.Context,
	queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration,
	scc models.SanctionCheckConfig,
	dataAccessor DataAccessor,
) (queriesOut []models.OpenSanctionsCheckQuery, emptyInput bool, err error) {
	queriesOut = queries
	nameFilterAny, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
		*scc.Query, iteration.OrganizationId,
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

	nameFilter, skip, preprocessingErr := e.preprocessCounterpartyName(ctx, nameFilter, scc.Preprocessing)
	if preprocessingErr != nil {
		err = preprocessingErr
		return
	}

	if !skip {
		queriesOut = append(queriesOut, models.OpenSanctionsCheckQuery{
			Type: "Thing",
			Filters: models.OpenSanctionCheckFilter{
				"name": []string{nameFilter},
			},
		})
	}

	return queriesOut, false, nil
}

func (e ScenarioEvaluator) evaluateSanctionCheckLabel(
	ctx context.Context,
	queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration,
	scc models.SanctionCheckConfig,
	dataAccessor DataAccessor,
) (queriesOut []models.OpenSanctionsCheckQuery, emptyInput bool, err error) {
	queriesOut = queries
	labelFilterAny, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
		*scc.Query, iteration.OrganizationId,
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
		return nil, false, errors.New("label filter name query did not return a string")
	}
	if labelFilter == "" {
		emptyInput = true
		return
	}

	processed, skip, preprocessingErr := e.preprocessCounterpartyNameForNer(ctx, labelFilter, scc.Preprocessing)
	if preprocessingErr != nil {
		err = preprocessingErr
		return
	}
	if skip {
		return nil, false, nil
	}

	labelFilter = processed

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

		return queriesOut, false, nil
	}

	matches, err := e.nameRecognizer.PerformNameRecognition(ctx, labelFilter)
	if err != nil {
		return queriesOut, false, errors.Wrap(err,
			"could not perform name recognition on label")
	}

	var personQuery *models.OpenSanctionsCheckQuery = nil
	var companyQuery *models.OpenSanctionsCheckQuery = nil

	if len(matches) == 0 {
		labelFilter, skip, preprocessingErr :=
			e.preprocessCounterpartyName(ctx, labelFilter, scc.Preprocessing)
		if preprocessingErr != nil {
			err = preprocessingErr
			return
		}

		if !skip {
			queriesOut = append(queriesOut, models.OpenSanctionsCheckQuery{
				Type:    "Thing",
				Filters: models.OpenSanctionCheckFilter{"name": []string{labelFilter}},
			})
		}
	}

	for _, match := range matches {
		labelFilter, skip, preprocessingErr :=
			e.preprocessCounterpartyName(ctx, match.Text, scc.Preprocessing)
		if preprocessingErr != nil {
			err = preprocessingErr
			return
		}
		if skip {
			continue
		}

		match.Text = labelFilter

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

	return queriesOut, false, nil
}

func (e ScenarioEvaluator) preprocessCounterpartyName(_ context.Context, input string,
	opts models.SanctionCheckConfigPreprocessing,
) (name string, skip bool, err error) {
	name = input

	if opts.RemoveNumbers {
		var tmp strings.Builder

		for _, c := range name {
			if !unicode.IsDigit(c) {
				tmp.WriteRune(c)
			}
		}

		name = tmp.String()
	}
	if opts.SkipIfUnder > 0 && len(input) < opts.SkipIfUnder {
		skip = true
		return
	}

	return
}

func (e ScenarioEvaluator) preprocessCounterpartyNameForNer(_ context.Context, input string,
	opts models.SanctionCheckConfigPreprocessing,
) (name string, skip bool, err error) {
	name = input

	if opts.SkipIfUnder > 0 && len(input) < opts.SkipIfUnder {
		skip = true
		return
	}

	return
}
