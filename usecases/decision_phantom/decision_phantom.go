package decision_phantom

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type TestRunEvaluator interface {
	EvalTestRunScenario(ctx context.Context, params evaluate_scenario.ScenarioEvaluationParameters) (se models.ScenarioExecution, err error)
}

type PhantomDecisionUsecaseSanctionCheckRepository interface {
	InsertSanctionCheck(
		ctx context.Context,
		exec repositories.Executor,
		phantomDecisionId string,
		sanctionCheck models.SanctionCheckWithMatches,
	) (models.SanctionCheckWithMatches, error)
}

type StoreTestRunRepository interface {
	StorePhantomDecision(
		ctx context.Context,
		exec repositories.Executor,
		decision models.PhantomDecision,
		organizationId string,
		testRunId string,
		newPhantomDecisionId string,
		scenarioVersion int,
	) error
}

type PhantomDecisionUsecase struct {
	enforceSecurity    security.EnforceSecurityPhantomDecision
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	repository         StoreTestRunRepository
	externalRepository PhantomDecisionUsecaseSanctionCheckRepository
	scenarioEvaluator  TestRunEvaluator
}

func NewPhantomDecisionUseCase(
	enforceSecurity security.EnforceSecurityPhantomDecision,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repository StoreTestRunRepository,
	extRepo PhantomDecisionUsecaseSanctionCheckRepository,
	scenarioEvaluator TestRunEvaluator,
) PhantomDecisionUsecase {
	return PhantomDecisionUsecase{
		enforceSecurity:    enforceSecurity,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repository:         repository,
		externalRepository: extRepo,
		scenarioEvaluator:  scenarioEvaluator,
	}
}

func (usecase *PhantomDecisionUsecase) CreatePhantomDecision(
	ctx context.Context,
	input models.CreatePhantomDecisionInput,
	evaluationParameters evaluate_scenario.ScenarioEvaluationParameters,
) (models.PhantomDecision, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"DecisionUsecase.CreatePhantomDecision",
		trace.WithAttributes(attribute.String("scenario_id", input.Scenario.Id)))
	defer span.End()

	if err := usecase.enforceSecurity.CreatePhantomDecision(input.OrganizationId); err != nil {
		return models.PhantomDecision{}, err
	}

	testRunScenarioExecution, err := usecase.scenarioEvaluator.EvalTestRunScenario(ctx, evaluationParameters)
	if err != nil {
		return models.PhantomDecision{},
			fmt.Errorf("error evaluating scenario: %w", err)
	}
	if testRunScenarioExecution.ScenarioId == "" {
		return models.PhantomDecision{}, nil
	}

	phantomDecision := models.AdaptScenarExecToPhantomDecision(testRunScenarioExecution)
	for i := range phantomDecision.RuleExecutions {
		phantomDecision.RuleExecutions[i].Evaluation = nil
	}
	ctx, span = tracer.Start(
		ctx,
		"DecisionUsecase.CreateDecision.store_phantom_decision",
		trace.WithAttributes(attribute.String("scenario_id", input.Scenario.Id)))
	defer span.End()

	err = usecase.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			if err = usecase.repository.StorePhantomDecision(
				ctx,
				tx,
				phantomDecision,
				input.OrganizationId,
				testRunScenarioExecution.TestRunId,
				phantomDecision.PhantomDecisionId,
				testRunScenarioExecution.ScenarioVersion,
			); err != nil {
				return err
			}

			if phantomDecision.SanctionCheckExecution != nil {
				// Clear matches to avoid storing them in the decision, we don't need them in the case of a phantom decision
				phantomDecision.SanctionCheckExecution.Matches = nil
				_, err := usecase.externalRepository.InsertSanctionCheck(
					ctx,
					tx,
					phantomDecision.PhantomDecisionId,
					*phantomDecision.SanctionCheckExecution)
				if err != nil {
					return errors.Wrap(err, "could not store sanction check execution")
				}
			}

			return nil
		},
	)
	if err != nil {
		return models.PhantomDecision{}, err
	}

	return phantomDecision, nil
}
