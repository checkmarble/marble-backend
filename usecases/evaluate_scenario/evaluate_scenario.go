package evaluate_scenario

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/cockroachdb/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

// Maximum number of rules executed concurrently
// TODO : set value from configuration/env instead
const MAX_CONCURRENT_RULE_EXECUTIONS = 5

type ScenarioEvaluationParameters struct {
	Scenario     models.Scenario
	ClientObject models.ClientObject
	DataModel    models.DataModel
	Pivot        *models.Pivot
}

type EvalScenarioRepository interface {
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (models.ScenarioIteration, error)
}

type ScenarioEvaluationRepositories struct {
	EvalScenarioRepository     EvalScenarioRepository
	ExecutorFactory            executor_factory.ExecutorFactory
	IngestedDataReadRepository repositories.IngestedDataReadRepository
	EvaluateAstExpression      ast_eval.EvaluateAstExpression
}

func EvalScenario(
	ctx context.Context,
	params ScenarioEvaluationParameters,
	repositories ScenarioEvaluationRepositories,
) (se models.ScenarioExecution, err error) {
	logger := utils.LoggerFromContext(ctx)
	start := time.Now()
	///////////////////////////////
	// Recover in case the evaluation panicked.
	// Even if there is a "recoverer" middleware in our stack, this allows a sentinel error to be used and to catch the failure early
	///////////////////////////////
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorContext(ctx, "recovered from panic during Eval. stacktrace from panic: ")
			logger.ErrorContext(ctx, string(debug.Stack()))

			err = models.ErrPanicInScenarioEvalution
			se = models.ScenarioExecution{}
		}
	}()

	logger.InfoContext(ctx, "Evaluating scenario", "scenarioId", params.Scenario.Id)

	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.EvalScenario")
	defer span.End()

	// If the scenario has no live version, don't try to Eval() it, return early
	if params.Scenario.LiveVersionID == nil {
		return models.ScenarioExecution{}, errors.Wrap(models.ErrScenarioHasNoLiveVersion,
			"scenario has no live version in EvalScenario")
	}

	liveVersion, err := repositories.EvalScenarioRepository.GetScenarioIteration(ctx,
		repositories.ExecutorFactory.NewExecutor(), *params.Scenario.LiveVersionID)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(err,
			"error getting scenario iteration in EvalScenario")
	}

	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(err,
			"error mapping published scenario iteration in eval scenario")
	}

	// Check the scenario & trigger_object's types
	if params.Scenario.TriggerObjectType != params.ClientObject.TableName {
		return models.ScenarioExecution{}, models.ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch
	}

	dataAccessor := DataAccessor{
		DataModel:                  params.DataModel,
		ClientObject:               params.ClientObject,
		executorFactory:            repositories.ExecutorFactory,
		organizationId:             params.Scenario.OrganizationId,
		ingestedDataReadRepository: repositories.IngestedDataReadRepository,
	}

	// Evaluate the trigger
	err = evalScenarioTrigger(
		ctx,
		repositories,
		publishedVersion.Body.TriggerConditionAstExpression,
		dataAccessor.organizationId,
		dataAccessor.ClientObject,
		params.DataModel,
	)
	if err != nil {
		return models.ScenarioExecution{}, err
	}

	// Evaluate all rules
	score, ruleExecutions, err := evalAllScenarioRules(ctx, repositories,
		publishedVersion.Body.Rules, dataAccessor, params.DataModel, logger)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(err,
			"error during concurrent rule evaluation")
	}

	pivotValue, err := getPivotValue(ctx, params.Pivot, dataAccessor)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(
			err,
			"error getting pivot value in EvalScenario")
	}

	// Compute outcome from score
	outcome := models.None

	if score < publishedVersion.Body.ScoreReviewThreshold {
		outcome = models.Approve
	} else if score < publishedVersion.Body.ScoreRejectThreshold {
		outcome = models.Review
	} else {
		outcome = models.Reject
	}

	// Build ScenarioExecution as result
	se = models.ScenarioExecution{
		ScenarioId:          params.Scenario.Id,
		ScenarioIterationId: publishedVersion.Id,
		ScenarioName:        params.Scenario.Name,
		ScenarioDescription: params.Scenario.Description,
		ScenarioVersion:     publishedVersion.Version,
		PivotValue:          pivotValue,
		RuleExecutions:      ruleExecutions,
		Score:               score,
		Outcome:             outcome,
		OrganizationId:      params.Scenario.OrganizationId,
	}
	if params.Pivot != nil {
		se.PivotId = &params.Pivot.Id
	}

	elapsed := time.Since(start)
	logger.InfoContext(ctx, fmt.Sprintf("Evaluated scenario in %dms", elapsed.Milliseconds()), "score", score, "outcome", outcome)

	return se, nil
}

func evalScenarioRule(
	ctx context.Context,
	repositories ScenarioEvaluationRepositories,
	rule models.Rule,
	dataAccessor DataAccessor,
	dataModel models.DataModel,
	logger *slog.Logger,
) (int, models.RuleExecution, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.evalScenarioRule",
		trace.WithAttributes(attribute.String("rule_id", rule.Id)))
	defer span.End()

	// return early if ctx is done
	select {
	case <-ctx.Done():
		return 0, models.RuleExecution{}, errors.Wrap(ctx.Err(),
			fmt.Sprintf("context cancelled when evaluating rule %s (%s)", rule.Name, rule.Id))
	default:
	}

	// Evaluate single rule
	returnValue, ruleEvaluation, err := repositories.EvaluateAstExpression.EvaluateAstExpression(
		ctx,
		*rule.FormulaAstExpression,
		dataAccessor.organizationId,
		dataAccessor.ClientObject,
		dataModel,
	)

	if err != nil && !ast.IsAuthorizedError(err) {
		return 0, models.RuleExecution{}, errors.Wrap(err,
			fmt.Sprintf("error while evaluating rule %s (%s)", rule.Name, rule.Id))
	}

	ruleEvaluationDto := ast.AdaptNodeEvaluationDto(ruleEvaluation)
	ruleExecution := models.RuleExecution{
		Rule:       rule,
		Evaluation: &ruleEvaluationDto,
		Result:     returnValue,
	}

	if err != nil {
		ruleExecution.Error = err
		logger.InfoContext(ctx, fmt.Sprintf("%v", ruleExecution.Error), //"Rule had an error",
			slog.String("ruleName", rule.Name),
			slog.String("ruleId", rule.Id),
		)
	}

	// Increment scenario score when rule is true
	if ruleExecution.Result {
		ruleExecution.ResultScoreModifier = rule.ScoreModifier
		logger.InfoContext(ctx, "Rule executed",
			slog.Int("score_modifier", rule.ScoreModifier),
			slog.String("ruleName", rule.Name),
			slog.Bool("result", ruleExecution.Result),
		)
	}
	return ruleExecution.ResultScoreModifier, ruleExecution, nil
}

func evalScenarioTrigger(
	ctx context.Context,
	repositories ScenarioEvaluationRepositories,
	triggerAstExpression ast.Node,
	organizationId string,
	payload models.ClientObject,
	dataModel models.DataModel,
) error {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.evalScenarioTrigger")
	defer span.End()

	returnValue, _, err := repositories.EvaluateAstExpression.EvaluateAstExpression(
		ctx,
		triggerAstExpression,
		organizationId,
		payload,
		dataModel,
	)
	isAuthorizedError := ast.IsAuthorizedError(err)
	if err != nil && !isAuthorizedError {
		return errors.Wrap(err,
			"Unexpected error evaluating trigger condition in EvalScenario")
	}

	if !returnValue || isAuthorizedError {
		return errors.Wrap(
			models.ErrScenarioTriggerConditionAndTriggerObjectMismatch,
			"scenario trigger object does not match payload in EvalScenario")
	}
	return nil
}

func evalAllScenarioRules(
	ctx context.Context,
	repositories ScenarioEvaluationRepositories,
	rules []models.Rule,
	dataAccessor DataAccessor,
	dataModel models.DataModel,
	logger *slog.Logger,
) (int, []models.RuleExecution, error) {
	// Results
	runningSumOfScores := 0
	ruleExecutions := make([]models.RuleExecution, len(rules))

	// Set max number of concurrent rule executions
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(MAX_CONCURRENT_RULE_EXECUTIONS)

	// Launch rules concurrently
	for i, rule := range rules {
		group.Go(func() error {
			// return early if ctx is done
			select {
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), fmt.Sprintf(
					"context cancelled before evaluating rule %s (%s)", rule.Name, rule.Id))
			default:
			}

			// Eval each rule
			scoreModifier, ruleExecution, err := evalScenarioRule(ctx, repositories, rule, dataAccessor, dataModel, logger)
			if err != nil {
				return err // First err will cancel the ctx
			}

			runningSumOfScores += scoreModifier
			ruleExecutions[i] = ruleExecution

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return 0, nil, fmt.Errorf("at least one rule evaluation returned an error: %w", err)
	}

	return runningSumOfScores, ruleExecutions, nil
}

func getPivotValue(ctx context.Context, pivot *models.Pivot, dataAccessor DataAccessor) (*string, error) {
	if pivot == nil {
		return nil, nil
	}

	var val any
	if len(pivot.PathLinks) == 0 {
		val = dataAccessor.ClientObject.Data[pivot.Field]
	} else {
		// This is V1. An optimized V2 would stop at the last but one step and use the last but one's link child field value (follow-up commit incoming)
		var err error
		val, err = dataAccessor.GetDbField(ctx, pivot.BaseTable, pivot.PathLinks, pivot.Field)
		if errors.Is(err, ast.ErrNullFieldRead) || errors.Is(err, ast.ErrNoRowsRead) {
			return nil, nil
		} else if err != nil {
			return nil, errors.Wrap(err, "error getting pivot value")
		}
	}

	if val == nil {
		return nil, nil
	}

	valStr, ok := val.(string)
	if !ok {
		return nil, errors.New("pivot value is not a string")
	}

	return &valStr, nil
}
