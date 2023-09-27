package repositories

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScenarioReadRepository interface {
	GetScenarioById(tx Transaction, scenarioId string) (models.Scenario, error)
	ListScenariosOfOrganization(tx Transaction, organizationId string) ([]models.Scenario, error)
	ListAllScenarios(tx Transaction) ([]models.Scenario, error)
}

type ScenarioReadRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func NewScenarioReadRepositoryPostgresql(transactionFactory TransactionFactoryPosgresql) ScenarioReadRepository {
	return &ScenarioReadRepositoryPostgresql{
		transactionFactory: transactionFactory,
	}
}

func selectScenarios() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioColumn...).
		From(dbmodels.TABLE_SCENARIOS)
}

func (repo *ScenarioReadRepositoryPostgresql) GetScenarioById(tx Transaction, scenarioId string) (models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		selectScenarios().Where(squirrel.Eq{"id": scenarioId}),
		FuncReturnsNilError(dbmodels.AdaptScenario),
	)
}

func (repo *ScenarioReadRepositoryPostgresql) ListScenariosOfOrganization(tx Transaction, organizationId string) ([]models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		selectScenarios().Where(squirrel.Eq{"org_id": organizationId}),
		FuncReturnsNilError(dbmodels.AdaptScenario),
	)
}

func (repo *ScenarioReadRepositoryPostgresql) ListAllScenarios(tx Transaction) ([]models.Scenario, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		selectScenarios().OrderBy("id"),
		FuncReturnsNilError(dbmodels.AdaptScenario),
	)
}
