package evaluate_scenario

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	// ErrSanctionCheckInvalidAst              = "invalid_ast"
	ErrSanctionCheckTriggerRuleNotBoolean   = "trigger_rule_not_boolean"
	ErrSanctionCheckCounterpartyIdNotString = "counterparty_id_not_string"
	ErrSanctionCheckAllFieldsNullOrEmpty    = "all_fields_null_or_empty"
	ErrSanctionCheckFieldsNotString         = "fields_not_string"
	ErrSanctionCheckPreprocessingFailed     = "preprocessing_failed"
)

func (e ScenarioEvaluator) evaluateSanctionCheck(
	ctx context.Context,
	iteration models.ScenarioIteration,
	params ScenarioEvaluationParameters,
	dataAccessor DataAccessor,
) (sexecs []models.SanctionCheckWithMatches, serr error) {
	// First, check if the sanction check should be performed
	if len(iteration.SanctionCheckConfigs) == 0 {
		return
	}

	var (
		wg   sync.WaitGroup
		lock sync.Mutex

		screeningExecutions = make([]models.SanctionCheckWithMatches, len(iteration.SanctionCheckConfigs))
		screeningErrors     = make([]error, 0, len(iteration.SanctionCheckConfigs))
	)

	addScreeningResult := func(idx int, result models.SanctionCheckWithMatches) {
		lock.Lock()
		defer lock.Unlock()

		screeningExecutions[idx] = result
	}

	addScreeningError := func(scc models.SanctionCheckConfig, err error) {
		lock.Lock()
		defer lock.Unlock()

		utils.LoggerFromContext(ctx).Error(fmt.Sprintf("screening execution returned some fatal errors: %s", err),
			"sanction_check_config_id", scc.Id)

		screeningErrors = append(screeningErrors, err)
	}

	for idx, scc := range iteration.SanctionCheckConfigs {
		wg.Add(1)

		go func(idx int, scc models.SanctionCheckConfig) {
			defer func() {
				if r := recover(); r != nil {
					utils.LoggerFromContext(ctx).ErrorContext(ctx, fmt.Sprintf("recovered from panic during screening execution: '%s'. stacktrace from panic:", r))
					utils.LogAndReportSentryError(ctx, errors.New(string(debug.Stack())))

					serr = models.ErrPanicInScenarioEvalution
				}

				wg.Done()
			}()

			scId := uuid.NewString()
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
					addScreeningError(scc, errors.New("could not parse screening trigger condition AST expression"))
					return
				}

				passed, ok := triggerEvaluation.ReturnValue.(bool)

				if !ok {
					addScreeningResult(idx, outcomeError(scc, ErrSanctionCheckTriggerRuleNotBoolean, nil))
					return
				}
				if !passed {
					addScreeningResult(idx, outcomeNoHit(scc))
					return
				}
			}

			var (
				queries []models.OpenSanctionsCheckQuery
				err     error
			)

			if scc.Query != nil {
				queries = []models.OpenSanctionsCheckQuery{
					{
						Type:    scc.EntityType,
						Filters: models.OpenSanctionCheckFilter{},
					},
				}

				for fieldName, fieldAst := range scc.Query {
					inputAst, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
						fieldAst, iteration.OrganizationId,
						dataAccessor.ClientObject, dataAccessor.DataModel)
					if err != nil {
						addScreeningError(scc, errors.New("could not parse screening counterparty name AST expression"))
						return
					}

					if inputAst.ReturnValue == nil {
						addScreeningResult(idx, outcomeError(scc, ErrSanctionCheckAllFieldsNullOrEmpty, nil))
						return
					}

					input, ok := inputAst.ReturnValue.(string)
					if !ok {
						addScreeningResult(idx, outcomeError(scc, ErrSanctionCheckFieldsNotString, nil))
						return
					}
					if input == "" {
						addScreeningResult(idx, outcomeError(scc, ErrSanctionCheckAllFieldsNullOrEmpty, nil))
						return
					}

					queries[0].Filters[fieldName] = []string{input}
				}

				if queries, err = e.preprocess(ctx, scId, queries, iteration, scc); err != nil {
					addScreeningResult(idx, outcomeError(scc, ErrSanctionCheckAllFieldsNullOrEmpty, nil))
					return
				}
			}

			if len(queries) == 0 {
				addScreeningResult(idx, outcomeNoHit(scc))
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
					addScreeningError(scc, errors.New("could not parse screening counterparty ID AST expression"))
					return
				}

				counterpartyId, ok := counterpartyIdResult.ReturnValue.(string)
				if counterpartyIdResult.ReturnValue == nil || !ok {
					addScreeningResult(idx, outcomeError(scc, ErrSanctionCheckCounterpartyIdNotString, nil))
					return
				}

				if trimmed := strings.TrimSpace(counterpartyId); trimmed != "" {
					uniqueCounterpartyIdentifier = &counterpartyId

					whitelistCount, err := e.evalSanctionCheckUsecase.CountWhitelistsForCounterpartyId(
						ctx, iteration.OrganizationId, *uniqueCounterpartyIdentifier)
					if err != nil {
						addScreeningError(scc, errors.Wrap(err, "could not retrieve whitelist count"))
						return
					}

					query.LimitIncrease = whitelistCount
				}
			}

			result, err := e.evalSanctionCheckUsecase.Execute(ctx, params.Scenario.OrganizationId, query)
			if err != nil {
				addScreeningError(scc, errors.Wrap(err, "could not perform sanction check"))
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

			result.Id = scId
			result.SanctionCheckConfigId = scc.Id
			result.Duration = time.Since(start)

			addScreeningResult(idx, result)
		}(idx, scc)
	}

	wg.Wait()

	if serr != nil {
		return
	}

	if len(screeningErrors) > 0 {
		serr = errors.Join(screeningErrors...)
		return
	}

	sexecs = screeningExecutions

	for _, sce := range sexecs {
		if sce.Status == models.SanctionStatusError {
			errStr := ""
			if sce.ErrorDetail != nil {
				errStr = sce.ErrorDetail.Error()
			}

			utils.LoggerFromContext(ctx).Warn("screening execution returned some errors",
				"sanction_check_config_id", sce.SanctionCheckConfigId,
				"sanction_check_id", sce.Id,
				"error_codes", sce.ErrorCodes,
				"error", errStr)
		}
	}

	return
}

func (e ScenarioEvaluator) preprocess(
	ctx context.Context,
	sanctionCheckId string,
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
		IgnoreList,
		SkipIfUnder,
	}

	for _, step := range steps {
		if out, err = step(ctx, e, sanctionCheckId, out, iteration, scc); err != nil {
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

func outcomeError(scc models.SanctionCheckConfig, code string, err error) models.SanctionCheckWithMatches {
	return models.SanctionCheckWithMatches{
		SanctionCheck: models.SanctionCheck{
			SanctionCheckConfigId: scc.Id,
			Status:                models.SanctionStatusError,
			ErrorCodes:            []string{code},
			ErrorDetail:           err,
		},
	}
}
