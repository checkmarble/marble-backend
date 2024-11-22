package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type ScenarioTestRunRepository interface {
	CreateTestRun(ctx context.Context, tx Transaction, testrunID string,
		input models.ScenarioTestRunInput) error
	GetActiveTestRunByScenarioIterationID(ctx context.Context, exec Executor,
		scenarioIterationID string) (*models.ScenarioTestRun, error)
	ListTestRunsByScenarioID(ctx context.Context, exec Executor, scenarioID string) ([]models.ScenarioTestRun, error)
	UpdateTestRunStatus(ctx context.Context, exec Executor,
		scenarioIterationID string, status models.TestrunStatus,
	) error
	GetTestRunByID(ctx context.Context, exec Executor, testrunID string) (*models.ScenarioTestRun, error)
}

func selectTestruns() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioTestRunColumns...).
		From(dbmodels.TABLE_SCENARIO_TESTRUN)
}

func (repo *MarbleDbRepository) CreateTestRun(ctx context.Context,
	tx Transaction, testrunID string, input models.ScenarioTestRunInput,
) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}
	err := ExecBuilder(
		ctx,
		tx,
		NewQueryBuilder().Insert(dbmodels.TABLE_SCENARIO_TESTRUN).
			Columns(
				"id",
				"scenario_iteration_id",
				"expires_at",
				"status",
			).
			Values(
				testrunID,
				input.ScenarioIterationId,
				time.Now().Add(input.Period),
				models.Pending.String(),
			),
	)
	if err != nil {
		return err
	}

	return nil
}

func (repo *MarbleDbRepository) UpdateTestRunStatus(ctx context.Context, exec Executor,
	scenarioIterationID string, status models.TestrunStatus,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Update(dbmodels.TABLE_SCENARIO_TESTRUN).Set("status", status.String()).Where(squirrel.Eq{
			"scenario_iteration_id": scenarioIterationID,
		}),
	)
	return err
}

func (repo *MarbleDbRepository) GetActiveTestRunByScenarioIterationID(ctx context.Context,
	exec Executor, scenarioIterationID string,
) (*models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := selectTestruns().Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationID}).Where(squirrel.Eq{
		"status": models.Up.String(),
	})
	return SqlToOptionalModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenarioTestrun,
	)
}

func (repo *MarbleDbRepository) ListTestRunsByScenarioID(ctx context.Context,
	exec Executor, scenarioID string,
) ([]models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SelectScenarioTestRunColumns...).
		From(dbmodels.TABLE_SCENARIO_TESTRUN + " AS tr").
		Join(dbmodels.TABLE_SCENARIO_ITERATIONS + " AS scit ON scit.id = tr.scenario_iteration_id").
		Join(dbmodels.TABLE_SCENARIOS + " AS sc ON sc.id = scit.scenario_id").
		Where(squirrel.Eq{"sc.id": scenarioID})
	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenarioTestrun,
	)
}

func (repo *MarbleDbRepository) GetTestRunByID(ctx context.Context, exec Executor, testrunID string) (*models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	return SqlToOptionalModel(
		ctx,
		exec,
		selectTestruns().Where(squirrel.Eq{"id": testrunID}),
		dbmodels.AdaptScenarioTestrun,
	)
}
