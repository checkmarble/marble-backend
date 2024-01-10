package evaluate_scenario

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/cockroachdb/errors"

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

type EvalScenarioRepository interface {
	GetScenarioIteration(ctx context.Context, tx repositories.Transaction, scenarioIterationId string) (models.ScenarioIteration, error)
}

type ScenarioEvaluationRepositories struct {
	EvalScenarioRepository     EvalScenarioRepository
	OrgTransactionFactory      transaction.Factory
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
	triggerPassed, err := repositories.EvaluateRuleAstExpression.EvaluateRuleAstExpression(
		ctx,
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
	score := 0
	ruleExecutions := make([]models.RuleExecution, 0)
	for _, rule := range publishedVersion.Body.Rules {

		scoreModifier, ruleExecution, err := evalScenarioRule(ctx, repositories, rule, dataAccessor, params.DataModel, logger)

		if err != nil {
			return models.ScenarioExecution{}, errors.Wrap(err, fmt.Sprintf("error evaluating rule %s (%s) in EvalScenario", rule.Name, rule.Id))
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

	elapsed := time.Since(start)
	logger.InfoContext(ctx, fmt.Sprintf("Evaluated scenario in %dms", elapsed.Milliseconds()), "score", score, "outcome", o)

	return se, nil
}

func evalScenarioRule(ctx context.Context, repositories ScenarioEvaluationRepositories, rule models.Rule, dataAccessor DataAccessor, dataModel models.DataModel, logger *slog.Logger) (int, models.RuleExecution, error) {
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
