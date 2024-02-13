package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) CreateScenario(ctx context.Context, tx Transaction_deprec, organizationId string, scenario models.CreateScenarioInput, newScenarioId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Insert(dbmodels.TABLE_SCENARIOS).
			Columns(
				"id",
				"org_id",
				"name",
				"description",
				"trigger_object_type",
			).
			Values(
				newScenarioId,
				organizationId,
				scenario.Name,
				scenario.Description,
				scenario.TriggerObjectType,
			),
	)
	if err != nil {
		return err
	}

	return nil
}

func (repo *MarbleDbRepository) UpdateScenario(ctx context.Context, tx Transaction_deprec, scenario models.UpdateScenarioInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIOS).
		Where("id = ?", scenario.Id)

	if scenario.Name != nil {
		sql = sql.Set("name", scenario.Name)
	}
	if scenario.Description != nil {
		sql = sql.Set("description", scenario.Description)
	}

	if _, err := pgTx.ExecBuilder(ctx, sql); err != nil {
		return err
	}

	return nil
}

func (repo *MarbleDbRepository) UpdateScenarioLiveIterationId(ctx context.Context, tx Transaction_deprec, scenarioId string, scenarioIterationId *string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIOS).
		Where("id = ?", scenarioId).
		Set("live_scenario_iteration_id", scenarioIterationId)

	if _, err := pgTx.ExecBuilder(ctx, sql); err != nil {
		return err
	}
	return nil
}
