package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		newPhantomDecisionId string,
		scenarioVersion int) error

	GetTestRunIterationIdByScenarioId(ctx context.Context, exec Executor, scenarioID string) (*string, error)
}

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
				ast.AdaptExecutionError(ruleExecution.Error),
				ruleExecution.Rule.Id,
				ruleExecution.Outcome,
			)
	}
	err = ExecBuilder(ctx, exec, builderForRules)
	return err
}

func (repo *MarbleDbRepository) GetTestRunIterationIdByScenarioId(ctx context.Context,
	exec Executor, scenarioID string,
) (*string, error) {
	// to be defined once we will integrate the testrun feature
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select("scit.id").
		From(dbmodels.TABLE_SCENARIO_ITERATIONS + " AS scit").
		Join(dbmodels.TABLE_SCENARIO_TESTRUN + " AS tr ON scit.id = tr.scenario_iteration_id").
		Join(dbmodels.TABLE_SCENARIOS + " AS sc ON sc.id = scit.scenario_id").
		Where(squirrel.Eq{"tr.status": models.Up.String()}).
		Where(squirrel.Eq{"sc.id": scenarioID})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	row := exec.QueryRow(ctx, sql, args...)
	var scenarioIterationID string
	err = row.Scan(&scenarioIterationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scenarioIterationID, nil
}
