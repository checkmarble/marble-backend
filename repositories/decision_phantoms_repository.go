package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DecisionPhantomUsecaseRepository interface {
	StorePhantomDecision(
		ctx context.Context,
		exec Executor,
		decision models.PhantomDecision,
		organizationId string,
		testRunId string,
		newPhantomDecisionId string) error

	GetTestRunIterationByScenarioId(ctx context.Context, exec Executor, scenarioID string) (models.ScenarioIteration, error)
}

func (repo *MarbleDbRepository) StorePhantomDecision(
	ctx context.Context,
	exec Executor,
	decision models.PhantomDecision,
	organizationId string,
	testRunId string,
	newPhantomDecisionId string,
) error {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"DecisionPhantomRepository.StorePhantomDecision.store_phantom_decision",
		trace.WithAttributes(attribute.String("phantom_decision_id", newPhantomDecisionId)))
	defer span.End()
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_PHANTOM_DECISIONS).
			Columns(
				"id",
				"org_id",
				"created_at",
				"outcome",
				"scenario_id",
				"scenario_iteration_id",
				"score",
				"test_run_id",
			).
			Values(
				newPhantomDecisionId,
				organizationId,
				decision.CreatedAt,
				decision.Outcome.String(),
				decision.ScenarioId,
				decision.ScenarioIterationId,
				decision.Score,
				testRunId,
			),
	)
	if err != nil {
		return err
	}

	ctx, span = tracer.Start(
		ctx,
		"DecisionPhantomRepository.StorePhantomDecision.store_phantom__decision_rules",
		trace.WithAttributes(attribute.String("phantom_decision_id", newPhantomDecisionId)))
	defer span.End()
	builderForRules := NewQueryBuilder().
		Insert(dbmodels.TABLE_DECISION_RULES).
		Columns(
			"id",
			"org_id",
			"decision_id",
			"score_modifier",
			"result",
			"error_code",
			"rule_id",
			"rule_evaluation",
			"outcome",
		)
	for _, ruleExecution := range decision.RuleExecutions {
		serializedRuleEvaluation, err := dbmodels.SerializeNodeEvaluationDto(ruleExecution.Evaluation)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("rule(%s):", ruleExecution.Rule.Id))
		}

		builderForRules = builderForRules.
			Values(
				uuid.Must(uuid.NewV7()).String(),
				organizationId,
				newPhantomDecisionId,
				ruleExecution.ResultScoreModifier,
				ruleExecution.Result,
				ast.AdaptExecutionError(ruleExecution.Error),
				ruleExecution.Rule.Id,
				serializedRuleEvaluation,
				ruleExecution.Outcome,
			)
	}
	err = ExecBuilder(ctx, exec, builderForRules)
	return err
}

func (repo *MarbleDbRepository) GetTestRunIterationByScenarioId(ctx context.Context,
	exec Executor, scenarioID string,
) (models.ScenarioIteration, error) {
	// to be defined once we will integrate the testrun feature
	return models.ScenarioIteration{}, nil
}
