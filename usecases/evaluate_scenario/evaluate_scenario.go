package evaluate_scenario

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type ScenarioEvaluationParameters struct {
	Scenario  models.Scenario
	Payload   models.PayloadReader
	DataModel models.DataModel
}

type ScenarioEvaluationRepositories struct {
	ScenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	OrgTransactionFactory           transaction.Factory
	IngestedDataReadRepository      repositories.IngestedDataReadRepository
	EvaluateRuleAstExpression       ast_eval.EvaluateRuleAstExpression
}

func EvalScenario(ctx context.Context, params ScenarioEvaluationParameters, repositories ScenarioEvaluationRepositories, logger *slog.Logger) (se models.ScenarioExecution, err error) {
	start := time.Now()
	///////////////////////////////
	// Recover in case the evaluation panicked.
	// Even if there is a "recoverer" middleware in our stack, this allows a sentinel error to be used and to catch the failure early
	///////////////////////////////
	defer func() {
		if r := recover(); r != nil {
			logger.WarnContext(ctx, "recovered from panic during Eval. stacktrace from panic: ")
			logger.WarnContext(ctx, string(debug.Stack()))

			err = models.PanicInScenarioEvalutionError
			se = models.ScenarioExecution{}
		}
	}()

	logger.InfoContext(ctx, "Evaluating scenario", "scenarioId", params.Scenario.Id)

	// If the scenario has no live version, don't try to Eval() it, return early
	if params.Scenario.LiveVersionID == nil {
		return models.ScenarioExecution{}, errors.Join(models.ScenarioHasNoLiveVersionError, models.BadParameterError)
	}

	liveVersion, err := repositories.ScenarioIterationReadRepository.GetScenarioIteration(nil, *params.Scenario.LiveVersionID)
	if err != nil {
		return models.ScenarioExecution{}, fmt.Errorf("error getting scenario iteration in eval scenar: %w", err)
	}

	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return models.ScenarioExecution{}, fmt.Errorf("error mapping published scenario iteration in eval scenario: %w", err)
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
	triggerPassed, err := repositories.EvaluateRuleAstExpression.EvaluateRuleAstExpression(
		publishedVersion.Body.TriggerConditionAstExpression,
		dataAccessor.organizationId,
		dataAccessor.Payload,
		params.DataModel,
	)

	if err != nil {
		return models.ScenarioExecution{}, fmt.Errorf("error evaluating trigger condition in eval scenario: %w", err)
	}
	if !triggerPassed {
		return models.ScenarioExecution{}, fmt.Errorf("error: scenario trigger object does not match payload %w; %w", models.BadParameterError, models.ScenarioTriggerConditionAndTriggerObjectMismatchError)
	}

	// Evaluate all rules
	score := 0
	ruleExecutions := make([]models.RuleExecution, 0)
	for _, rule := range publishedVersion.Body.Rules {

		scoreModifier, ruleExecution, err := evalScenarioRule(repositories, rule, dataAccessor, params.DataModel, logger)

		if err != nil {
			return models.ScenarioExecution{}, fmt.Errorf("error evaluating rule in eval scenario: %w", err)
		}
		score += scoreModifier
		ruleExecutions = append(ruleExecutions, ruleExecution)
	}

	// Compute outcome from score
	o := models.None

	if score < publishedVersion.Body.ScoreReviewThreshold {
		o = models.Approve
	}
	if score >= publishedVersion.Body.ScoreReviewThreshold && score < publishedVersion.Body.ScoreRejectThreshold {
		o = models.Review
	}
	if score > publishedVersion.Body.ScoreRejectThreshold {
		o = models.Reject
	}

	// Build ScenarioExecution as result
	se = models.ScenarioExecution{
		ScenarioId:          params.Scenario.Id,
		ScenarioName:        params.Scenario.Name,
		ScenarioDescription: params.Scenario.Description,
		ScenarioVersion:     publishedVersion.Version,
		RuleExecutions:      ruleExecutions,
		Score:               score,
		Outcome:             o,
	}

	logger.InfoContext(ctx, "Evaluated scenario", "score", score, "outcome", o)

	// print duration
	elapsed := time.Since(start)
	logger.InfoContext(ctx, "Evaluated scenario", "duration", elapsed.Milliseconds())
	return se, nil
}

func evalScenarioRule(repositories ScenarioEvaluationRepositories, rule models.Rule, dataAccessor DataAccessor, dataModel models.DataModel, logger *slog.Logger) (int, models.RuleExecution, error) {
	// Evaluate single rule

	ruleReturnValue, err := repositories.EvaluateRuleAstExpression.EvaluateRuleAstExpression(
		*rule.FormulaAstExpression,
		dataAccessor.organizationId,
		dataAccessor.Payload,
		dataModel,
	)

	isAuthorizedError := func(err error) bool {
		for _, authorizedError := range models.RuleExecutionAuthorizedErrors {
			if errors.Is(err, authorizedError) {
				return true
			}
		}
		return false
	}

	if err != nil && !isAuthorizedError(err) {
		return 0, models.RuleExecution{}, fmt.Errorf("error while evaluating rule %s: %w", rule.Name, err)
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
		logger.Info("Rule had an error",
			slog.String("ruleName", rule.Name),
			slog.String("ruleId", rule.Id),
			slog.String("error", ruleExecution.Error.Error()),
		)
	}

	// Increment scenario score when rule is true
	if ruleExecution.Result {
		logger.Info("Rule executed",
			slog.Int("score_modifier", rule.ScoreModifier),
			slog.String("ruleName", rule.Name),
			slog.Bool("result", ruleExecution.Result),
		)
		fmt.Printf("rule score modifier: %d\n", ruleExecution.Rule.ScoreModifier)
		score = ruleExecution.Rule.ScoreModifier
	}
	return score, ruleExecution, nil
}