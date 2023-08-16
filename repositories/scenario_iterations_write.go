package repositories

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
)

type ScenarioIterationWriteRepositoryLegacy interface {
	CreateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error)
	UpdateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error)
}

type ScenarioIterationWriteRepository interface {
	DeleteScenarioIteration(ctx context.Context, scenarioIterationId string) error
}

type ScenarioIterationWriteRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func (repo *ScenarioIterationWriteRepositoryPostgresql) DeleteScenarioIteration(ctx context.Context, scenarioIterationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(nil)

	_, err := pgTx.ExecBuilder(NewQueryBuilder().Delete(dbmodels.TABLE_SCENARIO_ITERATIONS).Where("id = ?", scenarioIterationId))
	return err
}
