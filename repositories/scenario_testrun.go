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
	GetByScenarioIterationID(ctx context.Context, exec Executor, scenarioID string) (models.ScenarioTestRun, error)
	GetByID(ctx context.Context, exec Executor, testrunID string) (models.ScenarioTestRun, error)
}

type ScenarioTestRunRepositoryPostgresql struct{}

func selectTestruns() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioTestRunColumns...).
		From(dbmodels.TABLE_SCENARIO_TESTRUN)
}

func (repo *ScenarioTestRunRepositoryPostgresql) CreateTestRun(ctx context.Context,
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
				models.Up.String(),
			),
	)
	if err != nil {
		return err
	}

	return nil
}

func (repo *ScenarioTestRunRepositoryPostgresql) GetByScenarioIterationID(ctx context.Context,
	exec Executor, scenarioIterationID string,
) (models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioTestRun{}, err
	}
	return SqlToModel(
		ctx,
		exec,
		selectTestruns().Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationID}),
		dbmodels.AdaptScenarioTestrun,
	)
}

func (repo *ScenarioTestRunRepositoryPostgresql) GetByID(ctx context.Context, exec Executor, testrunID string) (models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioTestRun{}, err
	}
	return SqlToModel(
		ctx,
		exec,
		selectTestruns().Where(squirrel.Eq{"id": testrunID}),
		dbmodels.AdaptScenarioTestrun,
	)
}
