package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PhantomDecisionUsecase struct {
	enforceSecurity            security.EnforceSecurityPhantomDecision
	transactionFactory         executor_factory.TransactionFactory
	executorFactory            executor_factory.ExecutorFactory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	repository                 repositories.DecisionPhantomUsecaseRepository
	evaluateAstExpression      ast_eval.EvaluateAstExpression
	snoozesReader              snoozesForDecisionReader
}

func (usecase *PhantomDecisionUsecase) CreatePhantomDecision(ctx context.Context,
	input models.CreatePhantomDecisionInput, evaluationParameters evaluate_scenario.ScenarioEvaluationParameters,
) (models.PhantomDecision, error) {
	exec := usecase.executorFactory.NewExecutor()
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"DecisionUsecase.CreatePhantomDecision",
		trace.WithAttributes(attribute.String("scenario_id", input.Scenario.Id)))
	defer span.End()
	if err := usecase.enforceSecurity.CreatePhantomDecision(input.OrganizationId); err != nil {
		return models.PhantomDecision{}, err
	}
	evaluationRepositories := evaluate_scenario.ScenarioEvaluationRepositories{
		EvalTestRunScenatioRepository: usecase.repository,
		ExecutorFactory:               usecase.executorFactory,
		IngestedDataReadRepository:    usecase.ingestedDataReadRepository,
		EvaluateAstExpression:         usecase.evaluateAstExpression,
		SnoozeReader:                  usecase.snoozesReader,
	}
	testRunScenarioExecution, err := evaluate_scenario.EvalTestRunScenario(ctx,
		evaluationParameters, evaluationRepositories)
	if err != nil {
		return models.PhantomDecision{},
			fmt.Errorf("error evaluating scenario: %w", err)
	}
	if testRunScenarioExecution.ScenarioId == "" {
		return models.PhantomDecision{}, err
	}
	decision := models.AdaptScenarExecToPhantomDecision(testRunScenarioExecution)
	for i := range decision.RuleExecutions {
		decision.RuleExecutions[i].Evaluation = nil
	}
	ctx, span = tracer.Start(
		ctx,
		"DecisionUsecase.CreateDecision.store_phantom_decision",
		trace.WithAttributes(attribute.String("scenario_id", input.Scenario.Id)))
	defer span.End()

	if err = usecase.repository.StorePhantomDecision(
		ctx,
		exec,
		decision,
		input.OrganizationId,
		testRunScenarioExecution.ScenarioIterationId,
		decision.PhantomDecisionId,
	); err != nil {
		return models.PhantomDecision{},
			fmt.Errorf("error storing phantom decision: %w", err)
	}
	return decision, nil
}
