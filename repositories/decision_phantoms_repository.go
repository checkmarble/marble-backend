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
		decision models.DecisionWithRuleExecutions,
		organizationId string,
		testRunId string,
		newDecisionId string) error

	GetTestRunIterationByScenarioId(ctx context.Context, exec Executor, scenarioID string) (models.ScenarioIteration, error)
}

func (repo *MarbleDbRepository) StorePhantomDecision(
	ctx context.Context,
	exec Executor,
	decision models.DecisionWithRuleExecutions,
	organizationId string,
	testRunId string,
	newDecisionId string,
) error {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"DecisionPhantomRepository.StorePhantomDecision.store_phantom_decision",
		trace.WithAttributes(attribute.String("decision_id", newDecisionId)),
		trace.WithAttributes(attribute.Int("nb_rule_executions", len(decision.RuleExecutions))))
	defer span.End()
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_DECISIONS).
			Columns(
				"id",
				"org_id",
				"created_at",
				"outcome",
				"pivot_id",
				"pivot_value",
				"review_status",
				"scenario_id",
				"scenario_iteration_id",
				"scenario_name",
				"scenario_description",
				"scenario_version",
				"score",
				"trigger_object",
				"trigger_object_type",
				"scheduled_execution_id",
				"test_run_id",
			).
			Values(
				newDecisionId,
				organizationId,
				decision.CreatedAt,
				decision.Outcome.String(),
				decision.PivotId,
				decision.PivotValue,
				decision.ReviewStatus,
				decision.ScenarioId,
				decision.ScenarioIterationId,
				decision.ScenarioName,
				decision.ScenarioDescription,
				decision.ScenarioVersion,
				decision.Score,
				decision.ClientObject.Data,
				decision.ClientObject.TableName,
				decision.ScheduledExecutionId,
				testRunId,
			),
	)
	if err != nil {
		return err
	}

	if len(decision.RuleExecutions) == 0 {
		return nil
	}

	ctx, span = tracer.Start(
		ctx,
		"DecisionPhantomRepository.StorePhantomDecision.store_phantom__decision_rules",
		trace.WithAttributes(attribute.String("decision_id", newDecisionId)),
		trace.WithAttributes(attribute.Int("nb_rule_executions", len(decision.RuleExecutions))))
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
				newDecisionId,
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
	return models.ScenarioIteration{}, nil
}
