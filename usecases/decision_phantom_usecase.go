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

type DecisionPhantomUsecase struct {
	enforceSecurity            security.EnforceSecurityPhantomDecision
	transactionFactory         executor_factory.TransactionFactory
	executorFactory            executor_factory.ExecutorFactory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	repository                 repositories.DecisionPhantomUsecaseRepository
	evaluateAstExpression      ast_eval.EvaluateAstExpression
	snoozesReader              snoozesForDecisionReader
}

func (usecase *DecisionPhantomUsecase) CreatePhantomDecision(ctx context.Context,
	input models.CreatePhantomDecisionInput, WithRuleExecutionDetails bool,
	evaluationParameters evaluate_scenario.ScenarioEvaluationParameters,
) (models.DecisionWithRuleExecutions, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"DecisionUsecase.CreatePhantomDecision",
		trace.WithAttributes(attribute.String("scenario_id", input.Scenario.Id)))
	defer span.End()
	if err := usecase.enforceSecurity.CreatePhantomDecision(input.OrganizationId); err != nil {
		return models.DecisionWithRuleExecutions{}, err
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
		return models.DecisionWithRuleExecutions{},
			fmt.Errorf("error evaluating scenario: %w", err)
	}
	if testRunScenarioExecution.ScenarioId != "" {
		decision := models.AdaptScenarExecToDecision(testRunScenarioExecution, input.Payload, nil)
		if !WithRuleExecutionDetails {
			for i := range decision.RuleExecutions {
				decision.RuleExecutions[i].Evaluation = nil
			}
		}
		ctx, span = tracer.Start(
			ctx,
			"DecisionUsecase.CreateDecision.store_phantom_decision",
			trace.WithAttributes(attribute.String("scenario_id", input.Scenario.Id)),
			trace.WithAttributes(attribute.Int("nb_rule_executions", len(decision.RuleExecutions))))
		defer span.End()

		newDecision, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
			tx repositories.Transaction,
		) (models.DecisionWithRuleExecutions, error) {
			if err = usecase.repository.StorePhantomDecision(
				ctx,
				tx,
				decision,
				input.OrganizationId,
				testRunScenarioExecution.ScenarioIterationId,
				decision.DecisionId,
			); err != nil {
				return models.DecisionWithRuleExecutions{},
					fmt.Errorf("error storing decision: %w", err)
			}

			// only refresh the decision if it has changed, meaning if it was added to a case
			return decision, nil
		})
		if err != nil {
			return models.DecisionWithRuleExecutions{}, err
		}
		return newDecision, nil
	}
	return models.DecisionWithRuleExecutions{}, err
}
