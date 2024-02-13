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
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
)

// Maximum number of rules executed concurrently
// TODO : set value from configuration/env instead
const MAX_CONCURRENT_RULE_EXECUTIONS = 5

type ScenarioEvaluationParameters struct {
	Scenario  models.Scenario
	Payload   models.PayloadReader
	DataModel models.DataModel
}

type EvalScenarioRepository interface {
	GetScenarioIteration(ctx context.Context, tx repositories.Transaction_deprec, scenarioIterationId string) (models.ScenarioIteration, error)
}

type ScenarioEvaluationRepositories struct {
	EvalScenarioRepository     EvalScenarioRepository
	OrgTransactionFactory      transaction.Factory_deprec
	IngestedDataReadRepository repositories.IngestedDataReadRepository
	EvaluateRuleAstExpression  ast_eval.EvaluateRuleAstExpression
}

func EvalScenario(ctx context.Context, params ScenarioEvaluationParameters, repositories ScenarioEvaluationRepositories, logger *slog.Logger) (se models.ScenarioExecution, err error) {
	start := time.Now()
	///////////////////////////////
	// Recover in case the evaluation panicked.
	// Even if there is a "recoverer" middleware in our stack, this allows a sentinel error to be used and to catch the failure early
	///////////////////////////////
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorContext(ctx, "recovered from panic during Eval. stacktrace from panic: ")
			logger.ErrorContext(ctx, string(debug.Stack()))

			err = models.PanicInScenarioEvalutionError
			se = models.ScenarioExecution{}
		}
	}()

	logger.InfoContext(ctx, "Evaluating scenario", "scenarioId", params.Scenario.Id)

	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.EvalScenario")
	defer span.End()

	// If the scenario has no live version, don't try to Eval() it, return early
	if params.Scenario.LiveVersionID == nil {
		return models.ScenarioExecution{}, errors.Wrap(models.ScenarioHasNoLiveVersionError, "scenario has no live version in EvalScenario")
	}

	liveVersion, err := repositories.EvalScenarioRepository.GetScenarioIteration(ctx, nil, *params.Scenario.LiveVersionID)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(err, "error getting scenario iteration in EvalScenario")
	}

	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(err, "error mapping published scenario iteration in eval scenario")
	}

	// Check the scenario & trigger_object's types
	if params.Scenario.TriggerObjectType != string(params.Payload.ReadTableName()) {
		return models.ScenarioExecution{}, models.ScenarioTriggerTypeAndTiggerObjectTypeMismatchError
	}

	dataAccessor := DataAccessor{
		DataModel:                  params.DataModel,
		Payload:                    params.Payload,
		orgTransactionFactory:      repositories.OrgTransactionFactory,
		organizationId:             params.Scenario.OrganizationId,
		ingestedDataReadRepository: repositories.IngestedDataReadRepository,
	}

	// Evaluate the trigger
	triggerPassed, err := evalScenarioTrigger(
		ctx,
		repositories,
		publishedVersion.Body.TriggerConditionAstExpression,
		dataAccessor.organizationId,
		dataAccessor.Payload,
		params.DataModel,
	)

	isAuthorizedError := models.IsAuthorizedError(err)
	if err != nil && !isAuthorizedError {
		return models.ScenarioExecution{}, errors.Wrap(err, "Unexpected error evaluating trigger condition in EvalScenario")
	}
	if !triggerPassed || isAuthorizedError {
		return models.ScenarioExecution{}, errors.Wrap(models.ScenarioTriggerConditionAndTriggerObjectMismatchError, "scenario trigger object does not match payload in EvalScenario")
	}

	// Evaluate all rules
	score, ruleExecutions, err := evalAllScenarioRules(ctx, repositories, publishedVersion.Body.Rules, dataAccessor, params.DataModel, logger)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(err, "error during concurrent rule evaluation")
	}

	//Compute outcome from score
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
		ScenarioName:        params.Scenario.Name,
		ScenarioDescription: params.Scenario.Description,
		ScenarioVersion:     publishedVersion.Version,
		RuleExecutions:      ruleExecutions,
		Score:               score,
		Outcome:             outcome,
	}

	elapsed := time.Since(start)
	logger.InfoContext(ctx, fmt.Sprintf("Evaluated scenario in %dms", elapsed.Milliseconds()), "score", score, "outcome", outcome)

	return se, nil
}

func evalScenarioRule(ctx context.Context, repositories ScenarioEvaluationRepositories, rule models.Rule, dataAccessor DataAccessor, dataModel models.DataModel, logger *slog.Logger) (int, models.RuleExecution, error) {

	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.evalScenarioRule", trace.WithAttributes(attribute.String("rule_id", rule.Id)))
	defer span.End()

	// return early if ctx is done
	select {
	case <-ctx.Done():
		return 0, models.RuleExecution{}, errors.Wrap(ctx.Err(), fmt.Sprintf("context cancelled when evaluating rule %s (%s)", rule.Name, rule.Id))
	default:
	}

	// Evaluate single rule
	ruleReturnValue, err := repositories.EvaluateRuleAstExpression.EvaluateRuleAstExpression(
		ctx,
		*rule.FormulaAstExpression,
		dataAccessor.organizationId,
		dataAccessor.Payload,
		dataModel,
	)

	if err != nil && !models.IsAuthorizedError(err) {
		return 0, models.RuleExecution{}, errors.Wrap(err, fmt.Sprintf("error while evaluating rule %s (%s)", rule.Name, rule.Id))
	}

	score := 0
	if ruleReturnValue {
		score = rule.ScoreModifier
	}

	ruleExecution := models.RuleExecution{
		Rule:                rule,
		Result:              ruleReturnValue,
		ResultScoreModifier: score,
	}

	if err != nil {
		ruleExecution.Rule = rule
		ruleExecution.Error = err
		logger.InfoContext(ctx, fmt.Sprintf("%v", ruleExecution.Error), //"Rule had an error",
			slog.String("ruleName", rule.Name),
			slog.String("ruleId", rule.Id),
		)
	}

	// Increment scenario score when rule is true
	if ruleExecution.Result {
		logger.InfoContext(ctx, "Rule executed",
			slog.Int("score_modifier", rule.ScoreModifier),
			slog.String("ruleName", rule.Name),
			slog.Bool("result", ruleExecution.Result),
		)
		score = ruleExecution.Rule.ScoreModifier
	}
	return score, ruleExecution, nil
}

func evalScenarioTrigger(ctx context.Context, repositories ScenarioEvaluationRepositories, ruleAstExpression ast.Node, organizationId string, payload models.PayloadReader, dataModel models.DataModel) (bool, error) {

	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.evalScenarioTrigger")
	defer span.End()

	return repositories.EvaluateRuleAstExpression.EvaluateRuleAstExpression(
		ctx,
		ruleAstExpression,
		organizationId,
		payload,
		dataModel,
	)
}

func evalAllScenarioRules(ctx context.Context, repositories ScenarioEvaluationRepositories, rules []models.Rule, dataAccessor DataAccessor, dataModel models.DataModel, logger *slog.Logger) (int, []models.RuleExecution, error) {

	// Results
	runningSumOfScores := 0
	ruleExecutions := make([]models.RuleExecution, len(rules))

	// Set max number of concurrent rule executions
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(MAX_CONCURRENT_RULE_EXECUTIONS)

	// Launch rules concurrently
	for i, rule := range rules {

		// i, rule := i, rule avoids scoping issues.
		// should be solved with go 1.22
		i, rule := i, rule
		group.Go(func() error {

			// return early if ctx is done
			select {
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), fmt.Sprintf("context cancelled before evaluating rule %s (%s)", rule.Name, rule.Id))
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
