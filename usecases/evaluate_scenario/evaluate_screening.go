package evaluate_scenario

import (
	"context"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/mohae/deepcopy"
)

const (
	ErrScreeningTriggerRuleNotBoolean   = "trigger_rule_not_boolean"
	ErrScreeningCounterpartyIdNotString = "counterparty_id_not_string"
	ErrScreeningAllFieldsNullOrEmpty    = "all_fields_null_or_empty"
	ErrScreeningFieldsNotString         = "fields_not_string"
	ErrScreeningPreprocessingFailed     = "preprocessing_failed"
)

func (e ScenarioEvaluator) evaluateScreening(
	ctx context.Context,
	iteration models.ScenarioIteration,
	params ScenarioEvaluationParameters,
	dataAccessor DataAccessor,
) (sexecs []models.ScreeningWithMatches, serr error) {
	// First, check if the screening should be performed
	if len(iteration.ScreeningConfigs) == 0 {
		return
	}

	var (
		wg   sync.WaitGroup
		lock sync.Mutex

		screeningExecutions = make([]models.ScreeningWithMatches, len(iteration.ScreeningConfigs))
		screeningErrors     = make([]error, 0, len(iteration.ScreeningConfigs))
	)

	addScreeningResult := func(idx int, result models.ScreeningWithMatches) {
		lock.Lock()
		defer lock.Unlock()

		screeningExecutions[idx] = result
	}

	addScreeningError := func(scc models.ScreeningConfig, err error) {
		lock.Lock()
		defer lock.Unlock()

		utils.LoggerFromContext(ctx).Error(fmt.Sprintf(
			"screening execution returned some fatal errors: %s", err),
			"screening_config_id", scc.Id)

		screeningErrors = append(screeningErrors, err)
	}

	for idx, scc := range iteration.ScreeningConfigs {
		wg.Add(1)

		go func(idx int, scc models.ScreeningConfig) {
			defer func() {
				if r := recover(); r != nil {
					utils.LoggerFromContext(ctx).ErrorContext(ctx,
						fmt.Sprintf("recovered from panic during screening execution: '%s'. stacktrace from panic:", r))
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
					addScreeningResult(idx, outcomeError(scc,
						ErrScreeningTriggerRuleNotBoolean, nil))
					return
				}
				if !passed {
					addScreeningResult(idx, outcomeNoHit(scc, nil))
					return
				}
			}

			var (
				queries                 []models.OpenSanctionsCheckQuery
				queriesBeforeProcessing []models.OpenSanctionsCheckQuery
				err                     error
			)

			if scc.Query != nil {
				queriesBeforeProcessing = []models.OpenSanctionsCheckQuery{
					{
						Type:    scc.EntityType,
						Filters: models.OpenSanctionsFilter{},
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
						addScreeningResult(idx, outcomeError(scc, ErrScreeningAllFieldsNullOrEmpty, nil))
						return
					}

					input, ok := inputAst.ReturnValue.(string)
					if !ok {
						addScreeningResult(idx, outcomeError(scc, ErrScreeningFieldsNotString, nil))
						return
					}
					if input == "" {
						addScreeningResult(idx, outcomeError(scc, ErrScreeningAllFieldsNullOrEmpty, nil))
						return
					}

					queriesBeforeProcessing[0].Filters[fieldName] = []string{input}
				}

				if queries, err = e.preprocess(ctx, scId, queriesBeforeProcessing, iteration, scc); err != nil {
					addScreeningResult(idx, outcomeError(scc, ErrScreeningAllFieldsNullOrEmpty, nil))
					return
				}
			}

			if len(queries) == 0 {
				addScreeningResult(idx, outcomeNoHit(scc, queriesBeforeProcessing))
				return
			}

			var uniqueCounterpartyIdentifier *string

			query := models.OpenSanctionsQuery{
				Config:  scc,
				Queries: queries,
			}

			if !reflect.DeepEqual(queries, queriesBeforeProcessing) {
				query.InitialQuery = queriesBeforeProcessing
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
					addScreeningResult(idx, outcomeError(scc,
						ErrScreeningCounterpartyIdNotString, nil))
					return
				}

				if trimmed := strings.TrimSpace(counterpartyId); trimmed != "" {
					uniqueCounterpartyIdentifier = &trimmed

					whitelistRecords, err := e.evalScreeningUsecase.SearchWhitelist(
						ctx,
						e.executorFactory.NewExecutor(),
						iteration.OrganizationId,
						uniqueCounterpartyIdentifier,
						nil,
						nil,
					)
					if err != nil {
						addScreeningError(
							scc,
							errors.Wrap(err, "could not retrieve whitelist records"),
						)
						return
					}
					whitelistIds := pure_utils.Map(
						whitelistRecords,
						func(item models.ScreeningWhitelist) string {
							return item.EntityId
						},
					)

					query.WhitelistedEntityIds = whitelistIds
				}
			}

			result, err := e.evalScreeningUsecase.Execute(ctx, params.Scenario.OrganizationId, query)
			if err != nil {
				addScreeningError(scc, errors.Wrap(err, "could not perform screening"))
				return
			}

			if uniqueCounterpartyIdentifier != nil {
				for idx := range result.Matches {
					result.Matches[idx].UniqueCounterpartyIdentifier = uniqueCounterpartyIdentifier
				}
			}

			result.Id = scId
			result.ScreeningConfigId = scc.Id
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
		if sce.Status == models.ScreeningStatusError {
			errStr := ""
			if sce.ErrorDetail != nil {
				errStr = sce.ErrorDetail.Error()
			}

			utils.LoggerFromContext(ctx).Warn("screening execution returned some errors",
				"screening_config_id", sce.ScreeningConfigId,
				"screening_id", sce.Id,
				"error_codes", sce.ErrorCodes,
				"error", errStr)
		}
	}

	return
}

func (e ScenarioEvaluator) preprocess(
	ctx context.Context,
	screeningId string,
	queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration,
	scc models.ScreeningConfig,
) ([]models.OpenSanctionsCheckQuery, error) {
	var err error

	out := deepcopy.Copy(queries).([]models.OpenSanctionsCheckQuery)

	steps := []ScreeningPreprocessor{
		SkipIfUnder,
		NameEntityRecognition,
		RemoveNumbers,
		IgnoreList,
		SkipIfUnder,
	}

	for _, step := range steps {
		if out, err = step(ctx, e, screeningId, out, iteration, scc); err != nil {
			return nil, err
		}

		if len(queries) == 0 {
			break
		}
	}

	return out, nil
}

func outcomeNoHit(scc models.ScreeningConfig, initialQuery []models.OpenSanctionsCheckQuery) models.ScreeningWithMatches {
	return models.ScreeningWithMatches{
		Screening: models.Screening{
			Config: models.ScreeningConfigRef{
				Id:       scc.Id,
				StableId: scc.StableId,
				Name:     scc.Name,
			},
			ScreeningConfigId: scc.Id,
			InitialQuery:      initialQuery,
			Status:            models.ScreeningStatusNoHit,
			ErrorCodes:        nil,
		},
	}
}

func outcomeError(scc models.ScreeningConfig, code string, err error) models.ScreeningWithMatches {
	return models.ScreeningWithMatches{
		Screening: models.Screening{
			Config: models.ScreeningConfigRef{
				Id:       scc.Id,
				StableId: scc.StableId,
				Name:     scc.Name,
			},
			ScreeningConfigId: scc.Id,
			Status:            models.ScreeningStatusError,
			ErrorCodes:        []string{code},
			ErrorDetail:       err,
		},
	}
}
