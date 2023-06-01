package usecases

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/utils"
	"runtime/debug"

	"golang.org/x/exp/slog"
)

type DecisionUsecase struct {
	dbPoolRepository                repositories.DbPoolRepository
	ingestedDataReadRepository      repositories.IngestedDataReadRepository
	decisionRepository              repositories.DecisionRepository
	datamodelRepository             repositories.DataModelRepository
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
}

func (usecase *DecisionUsecase) GetDecision(ctx context.Context, orgID string, decisionID string) (app.Decision, error) {
	return usecase.decisionRepository.GetDecision(ctx, orgID, decisionID)
}

func (usecase *DecisionUsecase) ListDecisions(ctx context.Context, orgID string) ([]app.Decision, error) {
	return usecase.decisionRepository.ListDecisions(ctx, orgID)
}

func (usecase *DecisionUsecase) CreateDecision(ctx context.Context, input models.CreateDecisionInput, logger *slog.Logger) (app.Decision, error) {
	scenario, err := usecase.scenarioReadRepository.GetScenario(ctx, input.OrganizationID, input.ScenarioID)
	if errors.Is(err, app.ErrNotFoundInRepository) {
		return app.Decision{}, fmt.Errorf("Scenario not found: %w", models.NotFoundError)
	} else if err != nil {
		return app.Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	dm, err := usecase.datamodelRepository.GetDataModel(ctx, input.OrganizationID)
	if errors.Is(err, app.ErrNotFoundInRepository) {
		return app.Decision{}, fmt.Errorf("Data model not found: %w", models.NotFoundError)
	} else if err != nil {
		return app.Decision{}, fmt.Errorf("error getting data model: %w", err)
	}

	scenarioExecution, err := usecase.EvalScenario(ctx, scenario, input.PayloadStructWithReader, dm, logger)
	if err != nil {
		return app.Decision{}, fmt.Errorf("error evaluating scenario: %w", err)
	}

	d := app.Decision{
		PayloadForArchive:   input.PayloadForArchive,
		Outcome:             scenarioExecution.Outcome,
		ScenarioID:          scenarioExecution.ScenarioID,
		ScenarioName:        scenarioExecution.ScenarioName,
		ScenarioDescription: scenarioExecution.ScenarioDescription,
		ScenarioVersion:     scenarioExecution.ScenarioVersion,
		RuleExecutions:      scenarioExecution.RuleExecutions,
		Score:               scenarioExecution.Score,
	}

	createdDecision, err := usecase.decisionRepository.StoreDecision(ctx, input.OrganizationID, d)
	if err != nil {
		return app.Decision{}, fmt.Errorf("error storing decision: %w", err)
	}

	return createdDecision, nil
}

func (usecase *DecisionUsecase) EvalScenario(ctx context.Context, scenario app.Scenario, payloadStructWithReader models.Payload, dataModel models.DataModel, logger *slog.Logger) (se app.ScenarioExecution, err error) {

	///////////////////////////////
	// Recover in case the evaluation panicked.
	// Even if there is a "recoverer" middleware in our stack, this allows a sentinel error to be used and to catch the failure early
	///////////////////////////////
	defer func() {
		if r := recover(); r != nil {
			logger.WarnCtx(ctx, "recovered from panic during Eval. stacktrace from panic: ")
			logger.WarnCtx(ctx, string(debug.Stack()))

			err = app.ErrPanicInScenarioEvalution
			se = app.ScenarioExecution{}
		}
	}()

	logger.InfoCtx(ctx, "Evaluting scenario", "scenarioId", scenario.ID)

	// If the scenario has no live version, don't try to Eval() it, return early
	if scenario.LiveVersionID == nil {
		return app.ScenarioExecution{}, app.ErrScenarioHasNoLiveVersion
	}

	orgID, err := utils.OrgIDFromCtx(ctx, nil)
	if err != nil {
		return app.ScenarioExecution{}, err
	}
	liveVersion, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(ctx, orgID, *scenario.LiveVersionID)
	if err != nil {
		return app.ScenarioExecution{}, err
	}

	publishedVersion, err := app.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return app.ScenarioExecution{}, err
	}

	// Check the scenario & trigger_object's types
	if scenario.TriggerObjectType != string(payloadStructWithReader.Table.Name) {
		return app.ScenarioExecution{}, app.ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch
	}

	dataAccessor := DataAccessor{DataModel: dataModel, Payload: payloadStructWithReader, dbPoolRepository: usecase.dbPoolRepository, ingestedDataReadRepository: usecase.ingestedDataReadRepository}

	// Evaluate the trigger
	triggerPassed, err := publishedVersion.Body.TriggerCondition.Eval(ctx, &dataAccessor)
	if err != nil {
		return app.ScenarioExecution{}, err
	}

	if !triggerPassed {
		return app.ScenarioExecution{}, app.ErrScenarioTriggerConditionAndTriggerObjectMismatch
	}

	// Evaluate all rules
	score := 0
	ruleExecutions := make([]app.RuleExecution, 0)
	for _, rule := range publishedVersion.Body.Rules {
		scoreModifier, ruleExecution, err := evalScenarioRule(ctx, rule, &dataAccessor, logger)
		if err != nil {
			return app.ScenarioExecution{}, err
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
	se = app.ScenarioExecution{
		ScenarioID:          scenario.ID,
		ScenarioName:        scenario.Name,
		ScenarioDescription: scenario.Description,
		ScenarioVersion:     publishedVersion.Version,
		RuleExecutions:      ruleExecutions,
		Score:               score,
		Outcome:             o,
	}

	logger.InfoCtx(ctx, "Evaluated scenario", "score", score, "outcome", o)

	return se, nil
}

func evalScenarioRule(ctx context.Context, rule app.Rule, dataAccessor operators.DataAccessor, logger *slog.Logger) (int, app.RuleExecution, error) {
	// Evaluate single rule
	score := 0
	ruleExecution, err := rule.Eval(ctx, dataAccessor)
	if err != nil {
		ruleExecution.Rule = rule
		ruleExecution, err = setRuleExecutionError(ruleExecution, err)
		if err != nil {
			return score, ruleExecution, err
		}
		logger.InfoCtx(ctx, "Rule had an error",
			slog.String("ruleName", rule.Name),
			slog.String("ruleId", rule.ID),
			slog.String("formula", rule.Formula.String()),
			slog.String("error", ruleExecution.Error.String()),
		)
	}

	// Increment scenario score when rule is true
	if ruleExecution.Result {
		logger.InfoCtx(ctx, "Rule executed",
			slog.Int("score_modifier", rule.ScoreModifier),
			slog.String("ruleName", rule.Name),
			slog.Bool("result", ruleExecution.Result),
		)
		fmt.Printf("rule score modifier: %d\n", ruleExecution.Rule.ScoreModifier)
		score = ruleExecution.Rule.ScoreModifier
	}
	return score, ruleExecution, nil
}

func setRuleExecutionError(ruleExecution app.RuleExecution, err error) (app.RuleExecution, error) {
	if errors.Is(err, models.OperatorNullValueReadError) {
		ruleExecution.Error = app.NullFieldRead
	} else if errors.Is(err, models.OperatorDivisionByZeroError) {
		ruleExecution.Error = app.DivisionByZero
	} else if errors.Is(err, models.OperatorNoRowsReadInDbError) {
		ruleExecution.Error = app.NoRowsRead
	} else {
		// return early in case of an unexpected error
		return ruleExecution, err
	}
	return ruleExecution, nil
}
