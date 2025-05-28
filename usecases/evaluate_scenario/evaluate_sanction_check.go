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
		emptyFieldRead := 0
		nbEvaluatedFields := 0
		emptyInput := false
		var err error

		if scc.Query != nil {
			nbEvaluatedFields += 1
			queries, emptyInput, err = e.evaluateSanctionCheckName(ctx, queries, iteration, scc, dataAccessor)
			if err != nil {
				return nil, true, errors.Wrap(err, "could not evaluate sanction check name")
			} else if emptyInput {
				emptyFieldRead += 1
			}
		}

		if emptyFieldRead == nbEvaluatedFields {
			sanctionCheck = append(sanctionCheck, models.SanctionCheckWithMatches{
				SanctionCheck: models.SanctionCheck{
					Status:     models.SanctionStatusError,
					ErrorCodes: []string{ErrSanctionCheckAllFieldsNullOrEmpty},
				},
			})

			performed = false
			return
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

	queriesOut = append(queriesOut, models.OpenSanctionsCheckQuery{
		Type: "Thing",
		Filters: models.OpenSanctionCheckFilter{
			"name": []string{nameFilter},
		},
	})

	return queriesOut, false, nil
}
