package repositories

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type ScenarioTestRunRepository interface {
	CreateTestRun(ctx context.Context, exec Executor, testrunID string,
		input models.ScenarioTestRunInput) error
	GetByScenarioIterationID(ctx context.Context, scenarioID string) (*models.ScenarioTestRun, error)
}

type ScenarioTestRunRepositoryPostgresql struct{}

func (repo *ScenarioTestRunRepositoryPostgresql) CreateTestRun(ctx context.Context,
	exec Executor, testrunID string, input models.ScenarioTestRunInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_SCENARIO_TESTRUN).
			Columns(
				"id",
				"scenario_iteration_id",
				"created_at",
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
