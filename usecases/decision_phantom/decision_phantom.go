package decision_phantom

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
	executorFactory            executor_factory.ExecutorFactory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	repository                 repositories.DecisionPhantomUsecaseRepository
	evaluateAstExpression      ast_eval.EvaluateAstExpression
	snoozesReader              evaluate_scenario.SnoozesForDecisionReader
}

func NewPhantomDecisionUseCase(enforceSecurity security.EnforceSecurityPhantomDecision,
	executorFactory executor_factory.ExecutorFactory, ingestedDataReadRepository repositories.IngestedDataReadRepository,
	repository repositories.DecisionPhantomUsecaseRepository, evaluateAstExpression ast_eval.EvaluateAstExpression,
	snoozesReader evaluate_scenario.SnoozesForDecisionReader,
) PhantomDecisionUsecase {
	return PhantomDecisionUsecase{
		enforceSecurity:            enforceSecurity,
		executorFactory:            executorFactory,
		ingestedDataReadRepository: ingestedDataReadRepository,
		repository:                 repository,
		evaluateAstExpression:      evaluateAstExpression,
		snoozesReader:              snoozesReader,
	}
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
		EvalTestRunScenarioRepository: usecase.repository,
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
		return models.PhantomDecision{}, nil
	}

	decision_phantom := models.AdaptScenarExecToPhantomDecision(testRunScenarioExecution)
	for i := range decision_phantom.RuleExecutions {
		decision_phantom.RuleExecutions[i].Evaluation = nil
	}
	ctx, span = tracer.Start(
		ctx,
		"DecisionUsecase.CreateDecision.store_phantom_decision",
		trace.WithAttributes(attribute.String("scenario_id", input.Scenario.Id)))
	defer span.End()

	if err = usecase.repository.StorePhantomDecision(
		ctx,
		exec,
		decision_phantom,
		input.OrganizationId,
		testRunScenarioExecution.ScenarioIterationId,
		decision_phantom.PhantomDecisionId,
		testRunScenarioExecution.ScenarioVersion,
	); err != nil {
		return models.PhantomDecision{},
			fmt.Errorf("error storing phantom decision: %w", err)
	}
	return decision_phantom, nil
}
