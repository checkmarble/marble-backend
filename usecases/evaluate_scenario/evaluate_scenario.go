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
// TODO: set value from configuration/env instead
const MAX_CONCURRENT_RULE_EXECUTIONS = 5

type ScenarioEvaluationParameters struct {
	Scenario          models.Scenario
	TargetIterationId *string
	ClientObject      models.ClientObject
	DataModel         models.DataModel
	Pivot             *models.Pivot
}

type SnoozesForDecisionReader interface {
	ListActiveRuleSnoozesForDecision(
		ctx context.Context,
		exec repositories.Executor,
		snoozeGroupIds []string,
		pivotValue string,
	) ([]models.RuleSnooze, error)
}

type ScenarioEvaluationRepositories struct {
	EvalScenarioRepository        repositories.EvalScenarioRepository
	EvalTestRunScenarioRepository repositories.EvalTestRunScenarioRepository
	ScenarioTestRunRepository     repositories.ScenarioTestRunRepository
	ScenarioRepository            repositories.ScenarioUsecaseRepository
	ExecutorFactory               executor_factory.ExecutorFactory
	IngestedDataReadRepository    repositories.IngestedDataReadRepository
	EvaluateAstExpression         ast_eval.EvaluateAstExpression
	SnoozeReader                  SnoozesForDecisionReader
}

func processScenarioIteration(ctx context.Context, params ScenarioEvaluationParameters,
	iteration models.ScenarioIteration, repositories ScenarioEvaluationRepositories, start time.Time,
	logger *slog.Logger, exec repositories.Executor,
) (models.ScenarioExecution, error) {
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

	errEval := evalScenarioTrigger(
		ctx,
		repositories,
		*iteration.TriggerConditionAstExpression,
		dataAccessor.organizationId,
		dataAccessor.ClientObject,
		params.DataModel,
	)
	if errEval != nil && !errors.Is(errEval, models.ErrScenarioTriggerConditionAndTriggerObjectMismatch) {
		return models.ScenarioExecution{}, errEval
	}
	var pivotValue *string
	var errPv error
	if params.Pivot != nil {
		pivotValue, errPv = getPivotValue(ctx, *params.Pivot, dataAccessor)
		if errPv != nil {
			return models.ScenarioExecution{}, errors.Wrap(
				errPv,
				"error getting pivot value in EvalScenario")
		}
	}

	snoozes := make([]models.RuleSnooze, 0)
	var errSnooze error
	if pivotValue != nil {
		snoozeGroupIds := make([]string, 0, len(iteration.Rules))
		for _, rule := range iteration.Rules {
			if rule.SnoozeGroupId != nil {
				snoozeGroupIds = append(snoozeGroupIds, *rule.SnoozeGroupId)
			}
		}
		snoozes, errSnooze = repositories.SnoozeReader.ListActiveRuleSnoozesForDecision(ctx, exec, snoozeGroupIds, *pivotValue)
	}
	if errSnooze != nil {
		return models.ScenarioExecution{}, errors.Wrap(
			errSnooze,
			"error when listing active rule snozze")
	}
	// Evaluate all rules
	score, ruleExecutions, errEval := evalAllScenarioRules(
		ctx,
		repositories,
		iteration.Rules,
		dataAccessor,
		params.DataModel,
		snoozes)
	if errEval != nil {
		return models.ScenarioExecution{}, errors.Wrap(errEval,
			"error during concurrent rule evaluation")
	}

	// Compute outcome from score
	var outcome models.Outcome

	if score >= *iteration.ScoreDeclineThreshold {
		outcome = models.Decline
	} else if score >= *iteration.ScoreBlockAndReviewThreshold {
		outcome = models.BlockAndReview
	} else if score >= *iteration.ScoreReviewThreshold {
		outcome = models.Review
	} else {
		outcome = models.Approve
	}

	// Build ScenarioExecution as result
	se := models.ScenarioExecution{
		ScenarioId:          params.Scenario.Id,
		ScenarioIterationId: iteration.Id,
		ScenarioName:        params.Scenario.Name,
		ScenarioDescription: params.Scenario.Description,
		ScenarioVersion:     *iteration.Version,
		RuleExecutions:      ruleExecutions,
		Score:               score,
		Outcome:             outcome,
		OrganizationId:      params.Scenario.OrganizationId,
	}
	if params.Pivot != nil {
		se.PivotId = &params.Pivot.Id
		se.PivotValue = pivotValue
	}

	elapsed := time.Since(start)
	logger.InfoContext(ctx, fmt.Sprintf("Evaluated scenario in %dms",
		elapsed.Milliseconds()), "score", score, "outcome", outcome)

	return se, nil
}

func EvalTestRunScenario(ctx context.Context,
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
	logger.InfoContext(ctx, "Evaluating scenario test run", "scenarioId", params.Scenario.Id)
	exec := repositories.ExecutorFactory.NewExecutor()
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.EvalTestRunScenario",
		trace.WithAttributes(
			attribute.String("scenario_id", params.Scenario.Id),
			attribute.String("organization_id", params.Scenario.OrganizationId),
			attribute.String("scenario_iteration_id", *params.Scenario.LiveVersionID),
			attribute.String("object_id", params.ClientObject.Data["object_id"].(string)),
		),
	)
	defer span.End()
	testrun, errTr := repositories.ScenarioTestRunRepository.GetTestRunByLiveVersionID(ctx, exec, *params.Scenario.LiveVersionID)
	if errTr != nil {
		return models.ScenarioExecution{}, errors.Wrap(errTr,
			"error getting testrun  in EvalTestRunScenario")
	}
	if testrun == nil || testrun.Status != models.Up {
		return models.ScenarioExecution{}, nil
	}
	scenario, errScenario := repositories.ScenarioRepository.GetScenarioByLiveScenarioIterationId(
		ctx, exec, testrun.ScenarioLiveIterationId)
	if errScenario != nil {
		return models.ScenarioExecution{}, errors.Wrap(errTr,
			"error getting scenario  in EvalTestRunScenario")
	}
	if scenario.Id == "" {
		logger.ErrorContext(ctx, "the live version iteration associated to the current testrun does not match with the actual live scenario iteration")
		return models.ScenarioExecution{}, nil
	}
	testRunIteration, err := repositories.EvalTestRunScenarioRepository.GetTestRunIterationByScenarioId(
		ctx, exec, params.Scenario.Id)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(err,
			"error getting testrun scenario iteration in EvalTestRunScenario")
	}
	if testRunIteration == nil {
		return models.ScenarioExecution{}, nil
	}
	se, errSe := processScenarioIteration(ctx, params, *testRunIteration, repositories, start, logger, exec)
	if errSe != nil {
		return models.ScenarioExecution{}, errors.Wrap(errSe,
			"error processing scenario iteration in EvalTestRunScenario")
	}
	return se, nil
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
	exec := repositories.ExecutorFactory.NewExecutor()

	// If the scenario has no live version, don't try to Eval() it, return early
	var targetVersionId string
	if params.TargetIterationId != nil {
		targetVersionId = *params.TargetIterationId
	} else if params.Scenario.LiveVersionID != nil {
		targetVersionId = *params.Scenario.LiveVersionID
	} else {
		return models.ScenarioExecution{}, errors.Wrap(models.ErrScenarioHasNoLiveVersion,
			"scenario has no live version in EvalScenario")
	}

	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.EvalScenario",
		trace.WithAttributes(
			attribute.String("scenario_id", params.Scenario.Id),
			attribute.String("organization_id", params.Scenario.OrganizationId),
			attribute.String("scenario_iteration_id", *params.Scenario.LiveVersionID),
			attribute.String("object_id", params.ClientObject.Data["object_id"].(string)),
		),
	)
	defer span.End()

	versionToRun, err := repositories.EvalScenarioRepository.GetScenarioIteration(ctx, exec, targetVersionId)
	if err != nil {
		return models.ScenarioExecution{}, errors.Wrap(err,
			"error getting scenario iteration in EvalScenario")
	}

	se, errSe := processScenarioIteration(ctx, params, versionToRun, repositories, start, logger, exec)
	if errSe != nil {
		return models.ScenarioExecution{}, errors.Wrap(errSe,
			"error processing scenario iteration in EvalTestRunScenario")
	}
	return se, nil
}

func evalScenarioRule(
	ctx context.Context,
	repositories ScenarioEvaluationRepositories,
	rule models.Rule,
	dataAccessor DataAccessor,
	dataModel models.DataModel,
	snoozes []models.RuleSnooze,
) (int, models.RuleExecution, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.evalScenarioRule",
		trace.WithAttributes(
			attribute.String("organization_id", rule.OrganizationId),
			attribute.String("rule_id", rule.Id),
			attribute.String("rule_name", rule.Name),
			attribute.String("scenario_iteration_id", rule.ScenarioIterationId),
		))
	defer span.End()
	logger := utils.LoggerFromContext(ctx)

	// return early if ctx is done
	select {
	case <-ctx.Done():
		return 0, models.RuleExecution{}, errors.Wrap(ctx.Err(),
			fmt.Sprintf("context cancelled when evaluating rule %s (%s)", rule.Name, rule.Id))
	default:
	}

	for _, snooze := range snoozes {
		if rule.SnoozeGroupId != nil && *rule.SnoozeGroupId == snooze.SnoozeGroupId {
			return 0, models.RuleExecution{Outcome: "snoozed", Rule: rule, Result: false}, nil
		}
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
		Outcome:    "no_hit",
		Rule:       rule,
		Evaluation: &ruleEvaluationDto,
		Result:     returnValue,
	}

	if err != nil {
		ruleExecution.Outcome = "error"
		ruleExecution.Error = err
		logger.InfoContext(ctx, fmt.Sprintf("%v", ruleExecution.Error), //"Rule had an error",
			slog.String("ruleName", rule.Name),
			slog.String("ruleId", rule.Id),
		)
	}

	// Increment scenario score when rule is true
	if ruleExecution.Result {
		ruleExecution.Outcome = "hit"
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
	snoozes []models.RuleSnooze,
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
			scoreModifier, ruleExecution, err := evalScenarioRule(ctx, repositories, rule, dataAccessor, dataModel, snoozes)
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

func getPivotValue(ctx context.Context, pivot models.Pivot, dataAccessor DataAccessor) (*string, error) {
	// In the case where a path through links is defined on the pivot, it's equivalent to stop at the penultimate link, because by hypothesis
	// of the join the child and parent field values are the same.
	// This allows us to do one fewer joins, and especially to return a value if the pivot object is not present (but the object "below" it is,
	// e.g. a transaction with its accountId is present but the account is not).
	// As a special case, if there is only one link to define the pivot value, we can just read the field value from the payload rather than
	// the ingested data.
	// This no longer works if we allow to define any field of the pivot object as the pivot value (currently it must be the last link's parent field)
	var val any
	links := dataAccessor.DataModel.AllLinksAsMap()
	if len(pivot.PathLinks) == 0 {
		val = dataAccessor.ClientObject.Data[pivot.Field]
	} else if len(pivot.PathLinks) == 1 {
		// special case of the below: we can read the field value from the payload
		link := links[pivot.PathLinkIds[0]]
		val = dataAccessor.ClientObject.Data[link.ChildFieldName]
	} else {
		lastLink := links[pivot.PathLinkIds[len(pivot.PathLinkIds)-1]]
		usefulLinks := pivot.PathLinks[:len(pivot.PathLinks)-1]
		var err error
		val, err = dataAccessor.GetDbField(ctx, pivot.BaseTable, usefulLinks, lastLink.ChildFieldName)
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
