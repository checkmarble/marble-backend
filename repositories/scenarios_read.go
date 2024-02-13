package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

func selectScenarios() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioColumn...).
		From(dbmodels.TABLE_SCENARIOS)
}

func (repo *MarbleDbRepository) GetScenarioById(ctx context.Context, tx Transaction_deprec, scenarioId string) (models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		selectScenarios().Where(squirrel.Eq{"id": scenarioId}),
		dbmodels.AdaptScenario,
	)
}

func (repo *MarbleDbRepository) ListScenariosOfOrganization(ctx context.Context, tx Transaction_deprec, organizationId string) ([]models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToListOfModels(
		ctx,
		pgTx,
		selectScenarios().Where(squirrel.Eq{"org_id": organizationId}).OrderBy("created_at DESC"),
		dbmodels.AdaptScenario,
	)
}

func (repo *MarbleDbRepository) ListAllScenarios(ctx context.Context, tx Transaction_deprec, filters models.ListAllScenariosFilters) ([]models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	query := selectScenarios().OrderBy("id")

	if filters.Live {
		query = query.Where(squirrel.NotEq{"live_scenario_iteration_id": nil})
	}

	return SqlToListOfModels(
		ctx,
		pgTx,
		query,
		dbmodels.AdaptScenario,
	)
}
