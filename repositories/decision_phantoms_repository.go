package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (repo *MarbleDbRepository) StorePhantomDecision(
	ctx context.Context,
	exec Executor,
	decision models.PhantomDecision,
	organizationId string,
	testRunId string,
	newPhantomDecisionId string,
	scenarioVersion int,
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
		NewQueryBuilder().
			Insert(dbmodels.TABLE_PHANTOM_DECISIONS).
			Columns(
				"id",
				"org_id",
				"created_at",
				"outcome",
				"scenario_id",
				"scenario_iteration_id",
				"score",
				"test_run_id",
				"scenario_version",
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
				scenarioVersion,
			),
	)
	if err != nil {
		return err
	}

	// It's possible that a scenario has no rules, just a sanction check config
	if len(decision.RuleExecutions) == 0 {
		return nil
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
			"outcome",
		)
	for _, ruleExecution := range decision.RuleExecutions {
		builderForRules = builderForRules.
			Values(
				uuid.Must(uuid.NewV7()).String(),
				organizationId,
				newPhantomDecisionId,
				ruleExecution.ResultScoreModifier,
				ruleExecution.Result,
				ruleExecution.ExecutionError,
				ruleExecution.Rule.Id,
				ruleExecution.Outcome,
			)
	}
	err = ExecBuilder(ctx, exec, builderForRules)
	return err
}
