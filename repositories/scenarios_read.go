package repositories

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

func selectScenarios() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioColumn...).
		From(dbmodels.TABLE_SCENARIOS)
}

func (repo *MarbleDbRepository) GetScenarioById(tx Transaction, scenarioId string) (models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		selectScenarios().Where(squirrel.Eq{"id": scenarioId}),
		dbmodels.AdaptScenario,
	)
}

func (repo *MarbleDbRepository) ListScenariosOfOrganization(tx Transaction, organizationId string) ([]models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		selectScenarios().Where(squirrel.Eq{"org_id": organizationId}),
		dbmodels.AdaptScenario,
	)
}

func (repo *MarbleDbRepository) ListAllScenarios(tx Transaction, filters models.ListAllScenariosFilters) ([]models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := selectScenarios().OrderBy("id")

	if filters.Live {
		query = query.Where(squirrel.NotEq{"live_scenario_iteration_id": nil})
	}

	return SqlToListOfModels(
		pgTx,
		query,
		dbmodels.AdaptScenario,
	)
}
