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
			inputAst, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
				scc.Query["name"], iteration.OrganizationId,
				dataAccessor.ClientObject, dataAccessor.DataModel)
			if err != nil {
				return
			}

			if inputAst.ReturnValue == nil {
				sanctionCheck = append(sanctionCheck, outcomeError(scc, ErrSanctionCheckAllFieldsNullOrEmpty))
				continue
			}

			input, ok := inputAst.ReturnValue.(string)
			if !ok {
				return nil, false, errors.New("name filter name query did not return a string")
			}
			if input == "" {
				sanctionCheck = append(sanctionCheck, outcomeError(scc, ErrSanctionCheckAllFieldsNullOrEmpty))
				continue
			}

			queries = []models.OpenSanctionsCheckQuery{
				{
					Type: "Thing",
					Filters: models.OpenSanctionCheckFilter{
						"name": []string{input},
					},
				},
			}

			if queries, err = e.preprocess(ctx, queries, iteration, scc); err != nil {
				return nil, true, errors.Wrap(err, "could not evaluate sanction check name")
			}
		}

		if len(queries) == 0 {
			sanctionCheck = append(sanctionCheck, outcomeNoHit(scc))
			continue
		}

		performed = true

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

func (e ScenarioEvaluator) preprocess(
	ctx context.Context,
	queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration,
	scc models.SanctionCheckConfig,
) ([]models.OpenSanctionsCheckQuery, error) {
	var err error

	out := append(make([]models.OpenSanctionsCheckQuery, 0, len(queries)), queries...)

	steps := []ScreeningPreprocessor{
		SkipIfUnder,
		NameEntityRecognition,
		RemoveNumbers,
		RemoveFromList,
		SkipIfUnder,
	}

	for _, step := range steps {
		if out, err = step(ctx, e, out, iteration, scc); err != nil {
			return nil, err
		}

		if len(queries) == 0 {
			break
		}
	}

	return out, nil
}

func outcomeNoHit(scc models.SanctionCheckConfig) models.SanctionCheckWithMatches {
	return models.SanctionCheckWithMatches{
		SanctionCheck: models.SanctionCheck{
			SanctionCheckConfigId: scc.Id,
			Status:                models.SanctionStatusNoHit,
			ErrorCodes:            []string{ErrSanctionCheckAllFieldsNullOrEmpty},
		},
	}
}

func outcomeError(scc models.SanctionCheckConfig, err string) models.SanctionCheckWithMatches {
	return models.SanctionCheckWithMatches{
		SanctionCheck: models.SanctionCheck{
			SanctionCheckConfigId: scc.Id,
			Status:                models.SanctionStatusError,
			ErrorCodes:            []string{err},
		},
	}
}
