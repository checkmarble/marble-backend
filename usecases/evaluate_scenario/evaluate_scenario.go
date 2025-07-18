package evaluate_scenario

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

// Maximum number of rules executed concurrently
// TODO: set value from configuration/env instead
const MAX_CONCURRENT_RULE_EXECUTIONS = 5

const (
	LogTriggerDurationKey    = "trigger"
	LogEvaluationDurationKey = "evaluation"
	LogRulesDurationKey      = "rules"
	LogScreeningDurationKey  = "screening"
	LogNerDurationKey        = "ner"
	LogStorageDurationKey    = "storage"
)

type ScenarioEvaluationParameters struct {
	Scenario          models.Scenario
	TargetIterationId *string
	ClientObject      models.ClientObject
	DataModel         models.DataModel
	Pivot             *models.Pivot
	CachedScreenings  map[string]models.ScreeningWithMatches
}

type EvalScreeningUsecase interface {
	Execute(context.Context, string, models.OpenSanctionsQuery) (models.ScreeningWithMatches, error)
	FilterOutWhitelistedMatches(context.Context, string, models.ScreeningWithMatches,
		string) (models.ScreeningWithMatches, error)
	CountWhitelistsForCounterpartyId(context.Context, string, string) (int, error)
}

type EvalNameRecognitionRepository interface {
	IsConfigured() bool
	PerformNameRecognition(context.Context, string) ([]httpmodels.HTTPNameRecognitionMatch, error)
}

type SnoozesForDecisionReader interface {
	ListActiveRuleSnoozesForDecision(
		ctx context.Context,
		exec repositories.Executor,
		snoozeGroupIds []string,
		pivotValue string,
	) ([]models.RuleSnooze, error)
}

type ScenarioEvaluatorFeatureAccessReader interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		organizationId string,
		userId *models.UserId,
	) (models.OrganizationFeatureAccess, error)
}

type EvaluateAstExpression interface {
	EvaluateAstExpression(
		ctx context.Context,
		cache *ast_eval.EvaluationCache,
		ruleAstExpression ast.Node,
		organizationId string,
		payload models.ClientObject,
		dataModel models.DataModel,
	) (ast.NodeEvaluation, error)
}

type ScenarioEvaluator struct {
	evalScenarioRepository        repositories.EvalScenarioRepository
	evalScreeningConfigRepository repositories.EvalScreeningConfigRepository
	evalScreeningUsecase          EvalScreeningUsecase
	scenarioTestRunRepository     repositories.ScenarioTestRunRepository
	scenarioRepository            repositories.ScenarioUsecaseRepository
	executorFactory               executor_factory.ExecutorFactory
	ingestedDataReadRepository    repositories.IngestedDataReadRepository
	evaluateAstExpression         EvaluateAstExpression
	snoozeReader                  SnoozesForDecisionReader
	featureAccessReader           ScenarioEvaluatorFeatureAccessReader
	nameRecognizer                EvalNameRecognitionRepository
}

func NewScenarioEvaluator(
	evalScenarioRepository repositories.EvalScenarioRepository,
	evalScreeningConfigRepository repositories.EvalScreeningConfigRepository,
	evalScreeningUsecase EvalScreeningUsecase,
	scenarioTestRunRepository repositories.ScenarioTestRunRepository,
	scenarioRepository repositories.ScenarioUsecaseRepository,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	evaluateAstExpression EvaluateAstExpression,
	snoozeReader SnoozesForDecisionReader,
	featureAccessReader ScenarioEvaluatorFeatureAccessReader,
	nameRecognitionRepository repositories.NameRecognitionRepository,
) ScenarioEvaluator {
	return ScenarioEvaluator{
		evalScenarioRepository:        evalScenarioRepository,
		evalScreeningConfigRepository: evalScreeningConfigRepository,
		evalScreeningUsecase:          evalScreeningUsecase,
		scenarioTestRunRepository:     scenarioTestRunRepository,
		scenarioRepository:            scenarioRepository,
		executorFactory:               executorFactory,
		ingestedDataReadRepository:    ingestedDataReadRepository,
		evaluateAstExpression:         evaluateAstExpression,
		snoozeReader:                  snoozeReader,
		featureAccessReader:           featureAccessReader,
		nameRecognizer:                nameRecognitionRepository,
	}
}

func (e ScenarioEvaluator) processScenarioIteration(
	ctx context.Context,
	params ScenarioEvaluationParameters,
	iteration models.ScenarioIteration,
	start time.Time,
	exec repositories.Executor,
) (bool, models.ScenarioExecution, error) {
	// Check the scenario & trigger_object's types
	if params.Scenario.TriggerObjectType != params.ClientObject.TableName {
		return false, models.ScenarioExecution{}, models.ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch
	}
	dataAccessor := DataAccessor{
		DataModel:                  params.DataModel,
		ClientObject:               params.ClientObject,
		executorFactory:            e.executorFactory,
		organizationId:             params.Scenario.OrganizationId,
		ingestedDataReadRepository: e.ingestedDataReadRepository,
	}

	cache := ast_eval.NewEvaluationCache()

	// Evaluate the trigger

	beforeTriggerExpression := time.Now()
	triggerExpressionDuration := 0 * time.Millisecond

	if iteration.TriggerConditionAstExpression != nil {
		ok, err := e.evalScenarioTrigger(
			ctx,
			cache,
			*iteration.TriggerConditionAstExpression,
			dataAccessor.organizationId,
			dataAccessor.ClientObject,
			params.DataModel,
		)
		if err != nil {
			return false, models.ScenarioExecution{}, errors.Wrap(err,
				"error evaluating trigger condition in EvalScenario")
		}
		if !ok {
			return false, models.ScenarioExecution{}, nil
		}

		triggerExpressionDuration = time.Since(beforeTriggerExpression)
	}

	var pivotValue *string
	var errPv error
	if params.Pivot != nil {
		pivotValue, errPv = getPivotValue(ctx, *params.Pivot, dataAccessor)
		if errPv != nil {
			return false, models.ScenarioExecution{}, errors.Wrap(
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
		snoozes, errSnooze = e.snoozeReader.ListActiveRuleSnoozesForDecision(ctx, exec, snoozeGroupIds, *pivotValue)
	}
	if errSnooze != nil {
		return false, models.ScenarioExecution{}, errors.Wrap(
			errSnooze,
			"error when listing active rule snozze")
	}

	beforeRules := time.Now()

	// Evaluate all rules
	score, ruleExecutions, errEval := e.evalAllScenarioRules(
		ctx,
		cache,
		iteration.Rules,
		dataAccessor,
		params.DataModel,
		snoozes)
	if errEval != nil {
		return false, models.ScenarioExecution{}, errors.Wrap(errEval,
			"error during concurrent rule evaluation")
	}

	rulesDuration := time.Since(beforeRules)

	var outcome models.Outcome

	screeningExecutions, err := e.evaluateScreening(ctx, iteration, params, dataAccessor)
	if err != nil {
		return false, models.ScenarioExecution{},
			errors.Wrap(err, "could not perform screening")
	}

	for idx, sce := range screeningExecutions {
		if sce.Count > 0 && iteration.ScreeningConfigs[idx].ForcedOutcome.Priority() > outcome.Priority() {
			outcome = iteration.ScreeningConfigs[idx].ForcedOutcome
		}
	}

	// We only go through the nominal score classifier if no screening returned any match (either no_hit or error)
	if !slices.ContainsFunc(screeningExecutions, func(e models.ScreeningWithMatches) bool {
		return e.Status != models.ScreeningStatusNoHit && e.Status != models.ScreeningStatusError
	}) {
		if score >= *iteration.ScoreDeclineThreshold {
			outcome = models.Decline
		} else if score >= *iteration.ScoreBlockAndReviewThreshold {
			outcome = models.BlockAndReview
		} else if score >= *iteration.ScoreReviewThreshold {
			outcome = models.Review
		} else {
			outcome = models.Approve
		}
	}

	scenarioID, err := uuid.Parse(params.Scenario.Id)
	if err != nil {
		return false, models.ScenarioExecution{}, errors.Wrap(err,
			"error parsing scenario id in EvalScenario")
	}
	scenarioIterationID, err := uuid.Parse(iteration.Id)
	if err != nil {
		return false, models.ScenarioExecution{}, errors.Wrap(err,
			"error parsing scenario iteration id in EvalScenario")
	}
	organizationID, err := uuid.Parse(params.Scenario.OrganizationId)
	if err != nil {
		return false, models.ScenarioExecution{}, errors.Wrap(err,
			"error parsing organization id in EvalScenario")
	}

	// Build ScenarioExecution as result
	se := models.ScenarioExecution{
		ScenarioId:          scenarioID,
		ScenarioIterationId: scenarioIterationID,
		ScenarioName:        params.Scenario.Name,
		ScenarioDescription: params.Scenario.Description,
		ScenarioVersion:     *iteration.Version,
		RuleExecutions:      ruleExecutions,
		ScreeningExecutions: screeningExecutions,
		Score:               score,
		Outcome:             outcome,
		OrganizationId:      organizationID,
	}
	if params.Pivot != nil {
		se.PivotId = &params.Pivot.Id
		se.PivotValue = pivotValue
	}

	elapsed := time.Since(start)

	ruleDurations := pure_utils.MapSliceToMap(ruleExecutions, func(exec models.RuleExecution) (string, int64) {
		id := exec.Rule.StableRuleId

		return id, exec.Duration.Milliseconds()
	})

	stepDurations := make(map[string]int64, 5)
	stepDurations[LogTriggerDurationKey] = triggerExpressionDuration.Milliseconds()
	stepDurations[LogEvaluationDurationKey] = elapsed.Milliseconds()
	stepDurations[LogRulesDurationKey] = rulesDuration.Milliseconds()

	for idx := range screeningExecutions {
		stepDurations[LogScreeningDurationKey] = screeningExecutions[idx].Duration.Milliseconds()

		if screeningExecutions[idx].NameRecognitionDuration != 0 {
			stepDurations[LogNerDurationKey] =
				screeningExecutions[idx].NameRecognitionDuration.Milliseconds()
		}
	}

	se.ExecutionMetrics = &models.ScenarioExecutionMetrics{
		Steps: stepDurations,
		Rules: ruleDurations,
	}

	return true, se, nil
}

func (e ScenarioEvaluator) EvalTestRunScenario(
	ctx context.Context,
	params ScenarioEvaluationParameters,
) (triggerPassed bool, se models.ScenarioExecution, err error) {
	logger := utils.LoggerFromContext(ctx)
	start := time.Now()
	///////////////////////////////
	// Recover in case the evaluation panicked.
	// Even if there is a "recoverer" middleware in our stack, this allows a sentinel error to be used and to catch the failure early
	///////////////////////////////
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorContext(ctx, "recovered from panic during EvalTestRunScenario. stacktrace from panic:")
			utils.LogAndReportSentryError(ctx, errors.New(string(debug.Stack())))

			err = models.ErrPanicInScenarioEvalution
			se = models.ScenarioExecution{}
		}
	}()
	logger.DebugContext(ctx, "Evaluating scenario test run", "scenarioId", params.Scenario.Id)
	exec := e.executorFactory.NewExecutor()
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.EvalTestRunScenario",
		trace.WithAttributes(
			attribute.String("scenario_id", params.Scenario.Id),
			attribute.String("organization_id", params.Scenario.OrganizationId),
			attribute.String("object_id", params.ClientObject.Data["object_id"].(string)),
		),
	)
	defer span.End()
	testruns, err := e.scenarioTestRunRepository.ListTestRunsByScenarioID(ctx, exec, params.Scenario.Id, models.Up)
	if err != nil {
		return false, se, err
	}
	if len(testruns) == 0 || testruns[0].Status != models.Up {
		return false, se, nil
	}

	if params.Scenario.LiveVersionID == nil || *params.Scenario.LiveVersionID != testruns[0].ScenarioLiveIterationId {
		logger.WarnContext(ctx, "the live version iteration associated to the current testrun does not match with the actual live scenario iteration")
		return false, se, nil
	}

	testRunIteration, err := e.evalScenarioRepository.GetScenarioIteration(ctx, exec, testruns[0].ScenarioIterationId)
	if err != nil {
		return false, se, err
	}

	// If the live version had a screening executed, and if it has the same configuration (except for the trigger rule),
	// we just reuse the cached screening execution to avoid another (possibly paid) call to the screening service.
	copiedScreening := make([]models.ScreeningWithMatches, 0)

	sccs, err := e.evalScreeningConfigRepository.ListScreeningConfigs(ctx, exec, testRunIteration.Id)
	if err != nil {
		return false, se, errors.Wrap(err,
			"error getting screening config from scenario iteration")
	}

	sccsToRun := slices.Clone(sccs)

	if len(sccs) > 0 {
		liveVersionSccs, err := e.evalScreeningConfigRepository.ListScreeningConfigs(
			ctx, exec, testruns[0].ScenarioLiveIterationId)
		if err != nil {
			return false, se, err
		}

		for _, testVersionScc := range sccs {
			for _, liveVersionScc := range liveVersionSccs {
				if testVersionScc.StableId == liveVersionScc.StableId {
					if cached, ok := params.CachedScreenings[liveVersionScc.ScenarioIterationId]; ok {
						if liveVersionScc.HasSameQuery(testVersionScc) {
							copiedScreening = append(copiedScreening, cached)
							sccsToRun = slices.Delete(sccsToRun, 0, 1)
							break
						}
					}
				}
			}
		}
	}

	testRunIteration.ScreeningConfigs = sccsToRun

	triggerPassed, se, err = e.processScenarioIteration(ctx, params, testRunIteration, start, exec)
	if err != nil {
		return false, se, err
	}
	if len(copiedScreening) > 0 {
		se.ScreeningExecutions = append(se.ScreeningExecutions, copiedScreening...)
	}
	se.TestRunId = testruns[0].Id
	return triggerPassed, se, nil
}

func (e ScenarioEvaluator) EvalScenario(
	ctx context.Context,
	params ScenarioEvaluationParameters,
) (triggerPassed bool, se models.ScenarioExecution, err error) {
	logger := utils.LoggerFromContext(ctx)
	start := time.Now()
	///////////////////////////////
	// Recover in case the evaluation panicked.
	// Even if there is a "recoverer" middleware in our stack, this allows a sentinel error to be used and to catch the failure early
	///////////////////////////////
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorContext(ctx, "recovered from panic during EvalScenario. stacktrace from panic:")
			utils.LogAndReportSentryError(ctx, errors.New(string(debug.Stack())))

			err = models.ErrPanicInScenarioEvalution
			se = models.ScenarioExecution{}
		}
	}()

	logger.DebugContext(ctx, "Evaluating scenario", "scenarioId", params.Scenario.Id)
	exec := e.executorFactory.NewExecutor()

	// If the scenario has no live version, don't try to Eval() it, return early
	var targetVersionId string
	if params.TargetIterationId != nil {
		targetVersionId = *params.TargetIterationId
	} else if params.Scenario.LiveVersionID != nil {
		targetVersionId = *params.Scenario.LiveVersionID
	} else {
		return false, models.ScenarioExecution{}, errors.Wrap(models.ErrScenarioHasNoLiveVersion,
			"scenario has no live version in EvalScenario")
	}

	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.EvalScenario",
		trace.WithAttributes(
			attribute.String("scenario_id", params.Scenario.Id),
			attribute.String("organization_id", params.Scenario.OrganizationId),
			attribute.String("scenario_iteration_id", targetVersionId),
			attribute.String("object_id", params.ClientObject.Data["object_id"].(string)),
		),
	)
	defer span.End()

	versionToRun, err := e.evalScenarioRepository.GetScenarioIteration(ctx, exec, targetVersionId)
	if err != nil {
		return false, models.ScenarioExecution{}, errors.Wrap(err,
			"error getting scenario iteration in EvalScenario")
	}

	scc, err := e.evalScreeningConfigRepository.ListScreeningConfigs(ctx, exec, versionToRun.Id)
	if err != nil {
		return false, models.ScenarioExecution{}, errors.Wrap(err,
			"error getting screening config from scenario iteration")
	}
	versionToRun.ScreeningConfigs = scc
	if len(scc) > 0 {
		featureAccess, err := e.featureAccessReader.GetOrganizationFeatureAccess(ctx, params.Scenario.OrganizationId, nil)
		if err != nil {
			return false, models.ScenarioExecution{}, err
		}
		if !featureAccess.Sanctions.IsAllowed() {
			return false, models.ScenarioExecution{}, errors.Wrapf(models.ForbiddenError,
				"screening feature access is missing: status is %s", featureAccess.Sanctions)
		}
	}

	triggerPassed, se, errSe := e.processScenarioIteration(ctx, params, versionToRun, start, exec)
	if errSe != nil {
		return false, models.ScenarioExecution{}, errors.Wrap(errSe,
			"error processing scenario iteration in EvalScenario")
	}
	return triggerPassed, se, nil
}

func (e ScenarioEvaluator) evalScenarioRule(
	ctx context.Context,
	cache *ast_eval.EvaluationCache,
	rule models.Rule,
	dataAccessor DataAccessor,
	dataModel models.DataModel,
	snoozes []models.RuleSnooze,
) (int, models.RuleExecution, error) {
	start := time.Now()
	ruleExecution := models.RuleExecution{}
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
		return 0, ruleExecution, errors.Wrap(ctx.Err(),
			fmt.Sprintf("context cancelled when evaluating rule %s (%s)", rule.Name, rule.Id))
	default:
	}

	for _, snooze := range snoozes {
		if rule.SnoozeGroupId != nil && *rule.SnoozeGroupId == snooze.SnoozeGroupId {
			return 0, models.RuleExecution{Outcome: "snoozed", Rule: rule, Result: false}, nil
		}
	}

	// Evaluate single rule
	returnValue := false
	hasError := false
	execErr := ast.NoError
	ruleEvaluation, err := e.evaluateAstExpression.EvaluateAstExpression(
		ctx,
		cache,
		*rule.FormulaAstExpression,
		dataAccessor.organizationId,
		dataAccessor.ClientObject,
		dataModel,
	)
	switch {
	// special errors are handled first
	case ast.IsAuthorizedError(err):
		execErr = ast.AdaptExecutionError(err)
		returnValue = false
		hasError = true
	case err != nil:
		return 0, ruleExecution, errors.Wrapf(err, "error while evaluating rule %s (%s)", rule.Name, rule.Id)
	case ruleEvaluation.ReturnValue == nil:
		execErr = ast.NullFieldRead
		hasError = true
		returnValue = false
	default:
		var ok bool
		returnValue, ok = ruleEvaluation.ReturnValue.(bool)
		if !ok {
			return 0, ruleExecution, errors.Wrapf(
				ast.ErrRuntimeExpression,
				"Unexpected error while evaluating rule %s (%s): rule returned a type %T",
				rule.Name, rule.Id, ruleEvaluation.ReturnValue)
		}
	}

	ruleEvaluationDto := ast.AdaptNodeEvaluationDto(ruleEvaluation)
	ruleExecution = models.RuleExecution{
		Outcome:    "no_hit",
		Rule:       rule,
		Evaluation: &ruleEvaluationDto,
		Result:     returnValue,
	}
	if hasError {
		ruleExecution.Outcome = "error"
		ruleExecution.ExecutionError = execErr
		logger.InfoContext(ctx, ruleExecution.ExecutionError.String(),
			slog.String("ruleName", rule.Name),
			slog.String("ruleId", rule.Id),
		)
	}

	ruleStats := ast.BuildEvaluationStats(ruleEvaluation, false)
	functionStats := ruleStats.FunctionStats()

	// Increment scenario score when rule result is true
	if ruleExecution.Result {
		ruleExecution.Outcome = "hit"
		ruleExecution.ResultScoreModifier = rule.ScoreModifier

		logger.DebugContext(ctx, fmt.Sprintf("rule evaluated in %dms",
			ruleStats.Took.Milliseconds()), "duration",
			ruleStats.Took.Milliseconds(), "nodes", ruleStats.Nodes, "skipped", ruleStats.SkippedCount,
			"cached", ruleStats.CachedCount, "ruleName", rule.Name, "score_modifier",
			rule.ScoreModifier, "result", ruleExecution.Result)

		logger.DebugContext(ctx, "rule nodes breakdown", "functions", functionStats)
	}

	ruleExecution.Duration = time.Since(start)

	return ruleExecution.ResultScoreModifier, ruleExecution, nil
}

func (e ScenarioEvaluator) evalScenarioTrigger(
	ctx context.Context,
	cache *ast_eval.EvaluationCache,
	triggerAstExpression ast.Node,
	organizationId string,
	payload models.ClientObject,
	dataModel models.DataModel,
) (bool, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "evaluate_scenario.evalScenarioTrigger")
	defer span.End()

	triggerEvaluation, err := e.evaluateAstExpression.EvaluateAstExpression(
		ctx,
		cache,
		triggerAstExpression,
		organizationId,
		payload,
		dataModel,
	)
	switch {
	case ast.IsAuthorizedError(err):
		return false, nil
	case err != nil:
		return false, errors.Wrap(err,
			"Unexpected error evaluating trigger condition in EvalScenario")
	case triggerEvaluation.ReturnValue == nil:
		return false, nil
	}

	boolReturnValue, ok := triggerEvaluation.ReturnValue.(bool)
	if !ok {
		return false, errors.Newf("root ast expression in trigger condition does not return a boolean, '%T' instead", triggerEvaluation.ReturnValue)
	}
	return boolReturnValue, nil
}

func (e ScenarioEvaluator) evalAllScenarioRules(
	ctx context.Context,
	cache *ast_eval.EvaluationCache,
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
			scoreModifier, ruleExecution, err := e.evalScenarioRule(ctx, cache, rule, dataAccessor, dataModel, snoozes)
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
		if err != nil {
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

func (e ScenarioEvaluator) EvalCaseName(
	ctx context.Context,
	params ScenarioEvaluationParameters,
	scenario models.Scenario,
	titleTemplate *ast.Node,
) (out string, err error) {
	out = fmt.Sprintf("Case for %s: %s", scenario.TriggerObjectType, params.ClientObject.Data["object_id"])

	if titleTemplate == nil {
		return
	}

	caseNameEvaluation, err := e.evaluateAstExpression.EvaluateAstExpression(
		ctx,
		nil,
		*titleTemplate,
		params.Scenario.OrganizationId,
		params.ClientObject,
		params.DataModel,
	)
	switch {
	case ast.IsAuthorizedError(err):
		return
	case err != nil:
		return "", errors.Wrap(err, "Unexpected error evaluating case name in EvalCaseName")
	case caseNameEvaluation.ReturnValue == nil:
		return
	}

	returnValue, ok := caseNameEvaluation.ReturnValue.(string)
	if !ok {
		return "", errors.Wrap(err, "case name query did not return a string")
	}
	if returnValue == "" {
		return
	}

	return returnValue, nil
}
